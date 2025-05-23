package disaster

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/InternalPointerVariable/ResQLink-Backend/internal/api"
)

type Server struct {
	repository Repository
	baseURL    string
}

func NewServer(repository Repository, baseURL string) *Server {
	return &Server{
		repository: repository,
		baseURL:    baseURL,
	}
}

func (s *Server) ListDisasterReportsByReporter(
	w http.ResponseWriter,
	r *http.Request,
) api.Response {
	ctx := r.Context()

	reporterID := r.PathValue("reporterId")

	reports, err := s.repository.ListDisasterReportsByReporter(ctx, reporterID)
	if err != nil {
		return api.Response{
			Error:   fmt.Errorf("get disaster reports by user: %w", err),
			Code:    http.StatusInternalServerError,
			Message: "Failed to get disaster reports.",
		}
	}

	return api.Response{
		Code:    http.StatusOK,
		Message: "Successfully fetched disaster reports.",
		Data:    reports,
	}
}

type createReportRequest struct {
	UserID       *string       `json:"userId"`
	Name         string        `json:"name"`
	Status       citizenStatus `json:"status"`
	RawSituation string        `json:"rawSituation"`
	PhotoURLs    []string      `json:"photoUrls"`
}

// NOTE: This is a version of `CreateDisasterReport` that uses `application/json`
// And it does not support image uploads
func (s *Server) CreateDisasterReportJson(w http.ResponseWriter, r *http.Request) api.Response {
	ctx := r.Context()

	var data createReportRequest

	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&data); err != nil {
		return api.Response{
			Error:   fmt.Errorf("create disaster report: %w", err),
			Code:    http.StatusBadRequest,
			Message: "Failed to create disaster report.",
		}
	}

	if err := s.repository.CreateDisasterReport(ctx, data); err != nil {
		return api.Response{
			Error:   fmt.Errorf("create disaster report: %w", err),
			Code:    http.StatusInternalServerError,
			Message: "Failed to create disaster report.",
		}
	}

	return api.Response{
		Code:    http.StatusCreated,
		Message: "Successfully created disaster report.",
	}
}

func (s *Server) CreateDisasterReport(w http.ResponseWriter, r *http.Request) api.Response {
	ctx := r.Context()

	const maxBodySize = 10 << 20
	r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)

	if err := r.ParseMultipartForm(maxBodySize); err != nil {
		return api.Response{
			Error:   fmt.Errorf("create disaster report: %w", err),
			Code:    http.StatusBadRequest,
			Message: "Failed to parse disaster report form data.",
		}
	}

	var userID *string
	userIDstr := r.FormValue("userId")
	if userIDstr != "" {
		userID = &userIDstr
	}

	disasterReport := createReportRequest{
		UserID:       userID,
		Name:         r.FormValue("name"),
		Status:       citizenStatus(r.FormValue("status")),
		RawSituation: r.FormValue("rawSituation"),
		PhotoURLs:    []string{},
	}

	if r.MultipartForm != nil && r.MultipartForm.File != nil {
		photos := r.MultipartForm.File["photos"]

		if len(photos) > 0 {
			for _, fileHeader := range photos {
				fileURL, err := uploadPhoto(fileHeader, s.baseURL)
				if err != nil {
					return api.Response{
						Error:   fmt.Errorf("create disaster report: %w", err),
						Code:    http.StatusInternalServerError,
						Message: "Failed to upload photo.",
					}
				}

				disasterReport.PhotoURLs = append(disasterReport.PhotoURLs, fileURL)
			}
		}
	}

	if err := s.repository.CreateDisasterReport(ctx, disasterReport); err != nil {
		return api.Response{
			Error:   fmt.Errorf("create disaster report: %w", err),
			Code:    http.StatusInternalServerError,
			Message: "Failed to create disaster report.",
		}
	}

	return api.Response{
		Code:    http.StatusCreated,
		Message: "Successfully created disaster report.",
	}
}

func (s *Server) ListDisasterReports(w http.ResponseWriter, r *http.Request) api.Response {
	ctx := r.Context()

	reports, err := s.repository.ListDisasterReports(ctx)
	if err != nil {
		return api.Response{
			Error:   fmt.Errorf("get disaster reports: %w", err),
			Code:    http.StatusInternalServerError,
			Message: "Failed to get disaster reports.",
		}
	}

	return api.Response{
		Code:    http.StatusOK,
		Message: "Successfully fetched disaster reports.",
		Data:    reports,
	}
}

func (s *Server) SetResponder(w http.ResponseWriter, r *http.Request) api.Response {
	ctx := r.Context()

	var data setResponderRequest

	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&data); err != nil {
		return api.Response{
			Error:   fmt.Errorf("set responder: %w", err),
			Code:    http.StatusBadRequest,
			Message: "Invalid set responder request.",
		}
	}

	reporterID := r.PathValue("reporterId")
	if data.ReporterID != reporterID {
		return api.Response{
			Error:   fmt.Errorf("set responder: reporter ID mismatch"),
			Code:    http.StatusBadRequest,
			Message: "Reporter ID in path and body doesn't match.",
		}
	}

	resp, err := s.repository.SetResponder(ctx, data)
	if err != nil {
		return api.Response{
			Error:   fmt.Errorf("set responder: %w", err),
			Code:    http.StatusBadRequest,
			Message: "Failed to set responder.",
		}
	}

	return api.Response{
		Code:    http.StatusOK,
		Data:    resp,
		Message: "Successfully set responder.",
	}
}
