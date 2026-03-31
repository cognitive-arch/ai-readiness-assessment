// internal/handlers/assessment.go
package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/yourorg/ai-readiness-backend/internal/audit"
	"github.com/yourorg/ai-readiness-backend/internal/metrics"
	"github.com/yourorg/ai-readiness-backend/internal/models"
	"github.com/yourorg/ai-readiness-backend/internal/repository"
	"github.com/yourorg/ai-readiness-backend/internal/service"
	"github.com/yourorg/ai-readiness-backend/internal/validator"
	"go.uber.org/zap"
)

// AssessmentHandler wires HTTP routes to the service layer.
type AssessmentHandler struct {
	svc   AssessmentServicer
	pdf   *service.PDFExporter
	audit *audit.Logger
	log   *zap.Logger
}

// NewAssessmentHandler constructs a handler with its dependencies.
func NewAssessmentHandler(
	svc AssessmentServicer,
	pdf *service.PDFExporter,
	auditLog *audit.Logger,
	log *zap.Logger,
) *AssessmentHandler {
	return &AssessmentHandler{svc: svc, pdf: pdf, audit: auditLog, log: log}
}

// POST /api/assessment
func (h *AssessmentHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateAssessmentRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	a, err := h.svc.CreateAssessment(r.Context(), req.ClientRef)
	if err != nil {
		h.log.Error("create assessment", zap.Error(err))
		respondError(w, http.StatusInternalServerError, "failed to create assessment")
		return
	}

	metrics.AssessmentsCreatedTotal.Inc()
	h.audit.AssessmentCreated(r.Context(), a.ID.Hex(), a.ClientRef, r.RemoteAddr)

	respondJSON(w, http.StatusCreated, toResponse(a))
}

// GET /api/assessment/{id}
func (h *AssessmentHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := validator.ValidateObjectID(id); err != nil {
		respondError(w, http.StatusNotFound, "assessment not found")
		return
	}

	a, err := h.svc.GetAssessment(r.Context(), id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			respondError(w, http.StatusNotFound, "assessment not found")
			return
		}
		h.log.Error("get assessment", zap.String("id", id), zap.Error(err))
		respondError(w, http.StatusInternalServerError, "failed to fetch assessment")
		return
	}
	respondJSON(w, http.StatusOK, toResponse(a))
}

// PUT /api/assessment/{id}/answers
func (h *AssessmentHandler) SaveAnswers(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := validator.ValidateObjectID(id); err != nil {
		respondError(w, http.StatusNotFound, "assessment not found")
		return
	}

	var req SaveAnswersRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	// Build valid ID set for validation
	bank := h.svc.QuestionBank()
	validIDs := make(map[string]struct{}, len(bank.Questions))
	for _, q := range bank.Questions {
		validIDs[q.ID] = struct{}{}
	}

	if err := validator.ValidateAnswers(req.Answers, validIDs); err != nil {
		if validator.IsValidationError(err) {
			ve := err.(*validator.ValidationError)
			respondValidationError(w, ve)
			return
		}
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	updated, err := h.svc.SaveAnswers(r.Context(), id, req.Answers)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			respondError(w, http.StatusNotFound, "assessment not found")
			return
		}
		h.log.Error("save answers", zap.String("id", id), zap.Error(err))
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	metrics.AnswersSavedTotal.Add(float64(len(req.Answers)))
	h.audit.AnswersSaved(r.Context(), id, len(req.Answers))

	respondJSON(w, http.StatusOK, toResponse(updated))
}

// POST /api/assessment/{id}/compute
func (h *AssessmentHandler) Compute(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := validator.ValidateObjectID(id); err != nil {
		respondError(w, http.StatusNotFound, "assessment not found")
		return
	}

	defer metrics.TimeScoringOp()()

	updated, err := h.svc.ComputeResult(r.Context(), id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			respondError(w, http.StatusNotFound, "assessment not found")
			return
		}
		h.log.Error("compute result", zap.String("id", id), zap.Error(err))
		respondError(w, http.StatusInternalServerError, "failed to compute result")
		return
	}

	if updated.Result != nil {
		metrics.AssessmentsComputedTotal.WithLabelValues(updated.Result.Maturity).Inc()
		for _, risk := range updated.Result.Risks {
			metrics.RiskFlagsTotal.WithLabelValues(risk).Inc()
		}
		h.audit.AssessmentComputed(r.Context(), id,
			updated.Result.Maturity,
			updated.Result.Overall,
			updated.Result.Risks,
		)
	}

	respondJSON(w, http.StatusOK, toResponse(updated))
}

// GET /api/assessment/{id}/results
func (h *AssessmentHandler) GetResults(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := validator.ValidateObjectID(id); err != nil {
		respondError(w, http.StatusNotFound, "assessment not found")
		return
	}

	result, err := h.svc.GetResult(r.Context(), id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			respondError(w, http.StatusNotFound, "assessment not found")
			return
		}
		respondError(w, http.StatusConflict, err.Error())
		return
	}

	h.audit.ResultFetched(r.Context(), id)
	respondJSON(w, http.StatusOK, result)
}

// GET /api/assessment/{id}/export/pdf
func (h *AssessmentHandler) ExportPDF(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := validator.ValidateObjectID(id); err != nil {
		respondError(w, http.StatusNotFound, "assessment not found")
		return
	}

	a, err := h.svc.GetAssessment(r.Context(), id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			respondError(w, http.StatusNotFound, "assessment not found")
			return
		}
		h.log.Error("export pdf: get assessment", zap.String("id", id), zap.Error(err))
		respondError(w, http.StatusInternalServerError, "failed to load assessment")
		return
	}

	if a.Result == nil {
		respondError(w, http.StatusConflict, "compute results before exporting PDF")
		return
	}

	path, err := h.pdf.Generate(a)
	if err != nil {
		h.log.Error("export pdf: generate", zap.String("id", id), zap.Error(err))
		metrics.PDFExportsTotal.WithLabelValues("failure").Inc()
		h.audit.PDFExported(r.Context(), id, false)
		respondError(w, http.StatusInternalServerError, "failed to generate PDF")
		return
	}
	defer service.CleanupFile(path)

	metrics.PDFExportsTotal.WithLabelValues("success").Inc()
	h.audit.PDFExported(r.Context(), id, true)

	f, err := os.Open(path)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to read generated PDF")
		return
	}
	defer f.Close()

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", `attachment; filename="ai-readiness-`+id+`.pdf"`)
	http.ServeContent(w, r, path, time.Now(), f)
}

// GET /api/assessment
func (h *AssessmentHandler) List(w http.ResponseWriter, r *http.Request) {
	rawLimit, _ := strconv.ParseInt(r.URL.Query().Get("limit"), 10, 64)
	rawOffset, _ := strconv.ParseInt(r.URL.Query().Get("offset"), 10, 64)

	limit, offset, err := validator.ValidatePagination(rawLimit, rawOffset)
	if err != nil {
		if validator.IsValidationError(err) {
			respondValidationError(w, err.(*validator.ValidationError))
			return
		}
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	assessments, total, err := h.svc.ListAssessments(r.Context(), limit, offset)
	if err != nil {
		h.log.Error("list assessments", zap.Error(err))
		respondError(w, http.StatusInternalServerError, "failed to list assessments")
		return
	}

	responses := make([]*AssessmentResponse, len(assessments))
	for i, a := range assessments {
		responses[i] = toResponse(a)
	}

	respondJSONWithMeta(w, http.StatusOK, responses, paginationMeta{
		Total:  total,
		Limit:  limit,
		Offset: offset,
	})
}

// DELETE /api/assessment/{id}
func (h *AssessmentHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := validator.ValidateObjectID(id); err != nil {
		respondError(w, http.StatusNotFound, "assessment not found")
		return
	}

	if err := h.svc.DeleteAssessment(r.Context(), id); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			respondError(w, http.StatusNotFound, "assessment not found")
			return
		}
		h.log.Error("delete assessment", zap.String("id", id), zap.Error(err))
		respondError(w, http.StatusInternalServerError, "failed to delete assessment")
		return
	}

	h.audit.AssessmentDeleted(r.Context(), id, r.RemoteAddr)
	respondJSON(w, http.StatusOK, map[string]string{"message": "deleted"})
}

// GET /api/questions
func (h *AssessmentHandler) GetQuestions(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, h.svc.QuestionBank())
}

// Response utilities
func toResponse(a *models.Assessment) *AssessmentResponse {
	return &AssessmentResponse{
		ID:        a.ID.Hex(),
		Status:    string(a.Status),
		Answers:   a.Answers,
		Result:    a.Result,
		CreatedAt: a.CreatedAt.Format(time.RFC3339),
		UpdatedAt: a.UpdatedAt.Format(time.RFC3339),
		ClientRef: a.ClientRef,
	}
}

func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(apiResponse{Success: true, Data: data})
}

func respondJSONWithMeta(w http.ResponseWriter, status int, data interface{}, meta interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(apiResponse{Success: true, Data: data, Meta: meta})
}

func respondError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(apiResponse{Success: false, Error: msg})
}

func respondValidationError(w http.ResponseWriter, ve *validator.ValidationError) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	_ = json.NewEncoder(w).Encode(struct {
		Success bool                   `json:"success"`
		Error   string                 `json:"error"`
		Errors  []validator.FieldError `json:"errors"`
	}{
		Success: false,
		Error:   "validation failed",
		Errors:  ve.Errors,
	})
}

func decodeJSON(r *http.Request, v interface{}) error {
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	return d.Decode(v)
}
