package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/finish06/drug-gate/internal/apikey"
	"github.com/finish06/drug-gate/internal/model"
	"github.com/go-chi/chi/v5"
)

// AdminHandler provides HTTP handlers for API key management.
type AdminHandler struct {
	store apikey.Store
}

// NewAdminHandler creates an AdminHandler with the given store.
func NewAdminHandler(store apikey.Store) *AdminHandler {
	return &AdminHandler{store: store}
}

type createKeyRequest struct {
	AppName   string   `json:"app_name"`
	Origins   []string `json:"origins"`
	RateLimit int      `json:"rate_limit"`
}

type rotateKeyRequest struct {
	GracePeriod string `json:"grace_period"`
}

type rotateKeyResponse struct {
	OldKey          string    `json:"old_key"`
	NewKey          string    `json:"new_key"`
	OldKeyExpiresAt time.Time `json:"old_key_expires_at"`
}

// CreateKey handles POST /admin/keys.
func (h *AdminHandler) CreateKey(w http.ResponseWriter, r *http.Request) {
	var req createKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAdminHandlerError(w, http.StatusBadRequest, "invalid_request", "Invalid JSON body")
		return
	}

	if req.AppName == "" {
		writeAdminHandlerError(w, http.StatusBadRequest, "validation_error", "app_name is required")
		return
	}
	if req.RateLimit <= 0 {
		writeAdminHandlerError(w, http.StatusBadRequest, "validation_error", "rate_limit must be greater than 0")
		return
	}

	ak, err := h.store.Create(r.Context(), req.AppName, req.Origins, req.RateLimit)
	if err != nil {
		slog.Error("failed to create API key", "err", err)
		writeAdminHandlerError(w, http.StatusInternalServerError, "internal_error", "Failed to create API key")
		return
	}

	slog.Info("api key created", "key", ak.Key, "app_name", ak.AppName)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(ak)
}

// ListKeys handles GET /admin/keys.
func (h *AdminHandler) ListKeys(w http.ResponseWriter, r *http.Request) {
	keys, err := h.store.List(r.Context())
	if err != nil {
		slog.Error("failed to list API keys", "err", err)
		writeAdminHandlerError(w, http.StatusInternalServerError, "internal_error", "Failed to list API keys")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(keys)
}

// GetKey handles GET /admin/keys/{key}.
func (h *AdminHandler) GetKey(w http.ResponseWriter, r *http.Request) {
	keyStr := chi.URLParam(r, "key")

	ak, err := h.store.Get(r.Context(), keyStr)
	if err != nil {
		slog.Error("failed to get API key", "err", err, "key", keyStr)
		writeAdminHandlerError(w, http.StatusInternalServerError, "internal_error", "Failed to get API key")
		return
	}
	if ak == nil {
		writeAdminHandlerError(w, http.StatusNotFound, "not_found", "API key not found")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(ak)
}

// DeactivateKey handles DELETE /admin/keys/{key}.
func (h *AdminHandler) DeactivateKey(w http.ResponseWriter, r *http.Request) {
	keyStr := chi.URLParam(r, "key")

	if err := h.store.Deactivate(r.Context(), keyStr); err != nil {
		slog.Error("failed to deactivate API key", "err", err, "key", keyStr)
		writeAdminHandlerError(w, http.StatusInternalServerError, "internal_error", "Failed to deactivate API key")
		return
	}

	slog.Info("api key deactivated", "key", keyStr)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "deactivated"})
}

// RotateKey handles POST /admin/keys/{key}/rotate.
func (h *AdminHandler) RotateKey(w http.ResponseWriter, r *http.Request) {
	oldKeyStr := chi.URLParam(r, "key")

	var req rotateKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAdminHandlerError(w, http.StatusBadRequest, "invalid_request", "Invalid JSON body")
		return
	}

	gracePeriod, err := time.ParseDuration(req.GracePeriod)
	if err != nil {
		writeAdminHandlerError(w, http.StatusBadRequest, "validation_error", "Invalid grace_period duration")
		return
	}

	newKey, err := h.store.Rotate(r.Context(), oldKeyStr, gracePeriod)
	if err != nil {
		slog.Error("failed to rotate API key", "err", err, "key", oldKeyStr)
		writeAdminHandlerError(w, http.StatusInternalServerError, "internal_error", "Failed to rotate API key")
		return
	}

	// Get the old key to read its expiration
	oldKey, _ := h.store.Get(r.Context(), oldKeyStr)
	var expiresAt time.Time
	if oldKey != nil && oldKey.ExpiresAt != nil {
		expiresAt = *oldKey.ExpiresAt
	}

	slog.Info("api key rotated", "old_key", oldKeyStr, "new_key", newKey.Key)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(rotateKeyResponse{
		OldKey:          oldKeyStr,
		NewKey:          newKey.Key,
		OldKeyExpiresAt: expiresAt,
	})
}

func writeAdminHandlerError(w http.ResponseWriter, status int, errCode, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(model.ErrorResponse{
		Error:   errCode,
		Message: message,
	})
}
