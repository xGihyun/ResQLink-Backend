package user

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/InternalPointerVariable/ResQLink-Backend/internal/api"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type Server struct {
	repository Repository
}

func NewServer(repository Repository) *Server {
	return &Server{
		repository: repository,
	}
}

type role string

const (
	citizen   role = "citizen"
	responder role = "responder"
)

type signUpRequest struct {
	Email                 string    `json:"email"`
	Password              string    `json:"password"`
	FirstName             string    `json:"firstName"`
	MiddleName            *string   `json:"middleName"`
	LastName              string    `json:"lastName"`
	BirthDate             time.Time `json:"birthDate"`
	Role                  role      `json:"role"`
	StatusUpdateFrequency uint      `json:"statusUpdateFrequency"`
	IsLocationShared      bool      `json:"isLocationShared"`
}

func (s *Server) SignUp(w http.ResponseWriter, r *http.Request) api.Response {
	ctx := r.Context()

	var data signUpRequest

	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&data); err != nil {
		return api.Response{
			Error:   fmt.Errorf("sign up: %w", err),
			Code:    http.StatusBadRequest,
			Message: "Invalid sign up request.",
		}
	}

	if err := s.repository.SignUp(ctx, data); err != nil {
		if pgErr, ok := err.(*pgconn.PgError); ok && pgErr.Code == "23505" {
			return api.Response{
				Error:   fmt.Errorf("sign up: %w", err),
				Code:    http.StatusConflict,
				Message: "User " + data.Email + " already exists.",
			}
		}

		return api.Response{
			Error:   fmt.Errorf("sign up: %w", err),
			Code:    http.StatusInternalServerError,
			Message: "Failed to sign up.",
		}
	}

	return api.Response{
		Code:    http.StatusCreated,
		Message: "Successfully signed up.",
	}
}

type BasicInfo struct {
	UserID     string  `json:"id"`
	FirstName  string  `json:"firstName"`
	MiddleName *string `json:"middleName"`
	LastName   string  `json:"lastName"`
}

type userResponse struct {
	BasicInfo

	CreatedAt             time.Time `json:"createdAt"`
	UpdatedAt             time.Time `json:"updatedAt"`
	Email                 string    `json:"email"`
	BirthDate             time.Time `json:"birthDate"`
	Role                  role      `json:"role"`
	StatusUpdateFrequency uint      `json:"statusUpdateFrequency"`
	IsLocationShared      bool      `json:"isLocationShared"`
}

type signInRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type signInResponse struct {
	User  userResponse `json:"user"`
	Token string       `json:"token"`
}

func (s *Server) SignIn(w http.ResponseWriter, r *http.Request) api.Response {
	ctx := r.Context()

	var data signInRequest

	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&data); err != nil {
		return api.Response{
			Error:   fmt.Errorf("sign in: %w", err),
			Code:    http.StatusBadRequest,
			Message: "Invalid sign in request.",
		}
	}

	response, err := s.repository.SignIn(ctx, data)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return api.Response{
				Error:   fmt.Errorf("sign in: %w", err),
				Code:    http.StatusNotFound,
				Message: "Invalid credentials.",
			}
		}

		if errors.Is(err, errInvalidPassword) {
			return api.Response{
				Error:   fmt.Errorf("sign in: %w", err),
				Code:    http.StatusUnauthorized,
				Message: "Invalid password.",
			}
		}

		return api.Response{
			Error:   fmt.Errorf("sign in: %w", err),
			Code:    http.StatusInternalServerError,
			Message: "Failed to sign in.",
		}
	}

	return api.Response{
		Code:    http.StatusOK,
		Message: "Successfully signed in.",
		Data:    response,
	}
}

func (s *Server) SignInAnonymous(w http.ResponseWriter, r *http.Request) api.Response {
	ctx := r.Context()

	var data signInAnonymousRequest

	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&data); err != nil {
		return api.Response{
			Error:   fmt.Errorf("sign in anon: %w", err),
			Code:    http.StatusBadRequest,
			Message: "Invalid anonymous sign in request.",
		}
	}

	response, err := s.repository.SignInAnonymous(ctx, data.AnonymousID)
	if err != nil {
		return api.Response{
			Error:   fmt.Errorf("sign in anon: %w", err),
			Code:    http.StatusInternalServerError,
			Message: "Failed to sign in as anonymous.",
		}
	}

	return api.Response{
		Code:    http.StatusOK,
		Message: "Successfully signed in as anonymous.",
		Data:    response,
	}
}

type signOutRequest struct {
	UserID string `json:"id"`
	Token  string `json:"token"`
}

func (s *Server) SignOut(w http.ResponseWriter, r *http.Request) api.Response {
	ctx := r.Context()

	var data signOutRequest

	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&data); err != nil {
		return api.Response{
			Error:   fmt.Errorf("sign out: %w", err),
			Code:    http.StatusBadRequest,
			Message: "Invalid sign out request.",
		}
	}

	if err := s.repository.invalidateSession(ctx, data.Token, data.UserID); err != nil {
		return api.Response{
			Error:   fmt.Errorf("sign out: %w", err),
			Code:    http.StatusInternalServerError,
			Message: "Failed to sign out.",
		}
	}

	return api.Response{
		Code:    http.StatusOK,
		Message: "Successfully signed out.",
	}
}

func (s *Server) GetSession(w http.ResponseWriter, r *http.Request) api.Response {
	ctx := r.Context()

	token := r.URL.Query().Get("token")

	result, err := s.repository.validateSessionToken(ctx, token)
	if err != nil {
		return api.Response{
			Error:   fmt.Errorf("get session: %w", err),
			Code:    http.StatusInternalServerError,
			Message: "Failed to get user session.",
		}
	}

	return api.Response{
		Code:    http.StatusOK,
		Message: "Successfully fetched user session.",
		Data:    result,
	}
}

func (s *Server) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		token, err := r.Cookie("session")
		if err != nil {
			slog.Error(err.Error())
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		_, err = s.repository.validateSessionToken(ctx, token.Value)
		if err != nil {
			slog.Error(err.Error())
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}
