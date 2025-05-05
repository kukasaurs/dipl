package handlers

import (
	"cleaning-app/cleaning-details-service/internal/models"
	"context"
	"encoding/json"
	"errors"
	"github.com/gorilla/mux"
	"net/http"
)

type CleaningServiceService interface {
	GetAllServices(context.Context) ([]models.CleaningService, error)
	GetActiveServices(context.Context) ([]models.CleaningService, error)
	CreateService(context.Context, *models.CleaningService) error
	UpdateService(context.Context, *models.CleaningService) error
	DeleteService(context.Context, string) error
	UpdateServiceStatus(context.Context, string, bool) error
}

type CleaningServiceHandler struct {
	service CleaningServiceService
}

func NewCleaningServiceHandler(service CleaningServiceService) *CleaningServiceHandler {
	return &CleaningServiceHandler{
		service: service,
	}
}

// GetAllServices gets all cleaning services (admin only)
func (h *CleaningServiceHandler) GetAllServices(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	services, err := h.service.GetAllServices(ctx)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err)
		return
	}

	respondWithJSON(w, http.StatusOK, services)
}

// GetActiveServices gets all active cleaning services (public endpoint)
func (h *CleaningServiceHandler) GetActiveServices(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	services, err := h.service.GetActiveServices(ctx)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err)
		return
	}

	respondWithJSON(w, http.StatusOK, services)
}

// CreateService creates a new cleaning service
func (h *CleaningServiceHandler) CreateService(w http.ResponseWriter, r *http.Request) {
	service := new(models.CleaningService)
	if err := json.NewDecoder(r.Body).Decode(service); err != nil {
		respondWithError(w, http.StatusBadRequest, err)
		return
	}

	ctx := r.Context()
	if err := h.service.CreateService(ctx, service); err != nil {
		if errors.Is(err, models.ErrDuplicate) {
			respondWithError(w, http.StatusConflict, err)
			return
		}
		respondWithError(w, http.StatusBadRequest, err)
		return
	}

	respondWithJSON(w, http.StatusCreated, service)
}

// UpdateService updates a cleaning service
func (h *CleaningServiceHandler) UpdateService(w http.ResponseWriter, r *http.Request) {
	service := new(models.CleaningService)
	if err := json.NewDecoder(r.Body).Decode(service); err != nil {
		respondWithError(w, http.StatusBadRequest, err)
		return
	}

	ctx := r.Context()
	if err := h.service.UpdateService(ctx, service); err != nil {
		handleServiceError(w, err)
		return
	}

	respondWithJSON(w, http.StatusOK, service)
}

// DeleteService deletes a cleaning service
func (h *CleaningServiceHandler) DeleteService(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	if id == "" {
		respondWithError(w, http.StatusBadRequest, errors.New("invalid id"))
		return
	}

	ctx := r.Context()
	if err := h.service.DeleteService(ctx, id); err != nil {
		handleServiceError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ToggleServiceStatus updates a cleaning service's active status
func (h *CleaningServiceHandler) ToggleServiceStatus(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	if id == "" {
		respondWithError(w, http.StatusBadRequest, errors.New("invalid id"))
		return
	}

	statusUpdate := new(models.ServiceStatusUpdate)
	if err := json.NewDecoder(r.Body).Decode(statusUpdate); err != nil {
		respondWithError(w, http.StatusBadRequest, err)
		return
	}

	ctx := r.Context()
	if err := h.service.UpdateServiceStatus(ctx, id, statusUpdate.IsActive); err != nil {
		handleServiceError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// Helper functions
func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(payload)
}

func respondWithError(w http.ResponseWriter, code int, err error) {
	respondWithJSON(w, code, map[string]string{"error": err.Error()})
}

func handleServiceError(w http.ResponseWriter, err error) {
	if errors.Is(err, models.ErrNotFound) {
		respondWithError(w, http.StatusNotFound, err)
		return
	}

	if errors.Is(err, models.ErrInvalidID) {
		respondWithError(w, http.StatusBadRequest, err)
		return
	}

	if errors.Is(err, models.ErrDuplicate) {
		respondWithError(w, http.StatusConflict, err)
		return
	}

	respondWithError(w, http.StatusInternalServerError, err)
}
