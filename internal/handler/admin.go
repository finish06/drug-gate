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
//
// @Summary      Create API key
// @Description  Provisions a new publishable API key with app name, allowed origins, and rate limit.
// @Tags         admin
// @Accept       json
// @Produce      json
// @Param        body  body  createKeyRequest  true  "Key creation parameters"
// @Success      201  {object}  apikey.APIKey
// @Failure      400  {object}  model.ErrorResponse  "Invalid request body or validation error"
// @Failure      500  {object}  model.ErrorResponse  "Internal error"
// @Security     AdminAuth
// @Router       /admin/keys [post]
func (h *AdminHandler) CreateKey(w http.ResponseWriter, r *http.Request) {
	var req createKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAdminHandlerError(w, http.StatusBadRequest, "invalid_request", "Invalid JSON body")
		return
	}

	if req.AppName == "" {
		writeAdminHandlerError(w, http.StatusBadRequest, "bad_request", "app_name is required")
		return
	}
	if req.RateLimit <= 0 {
		writeAdminHandlerError(w, http.StatusBadRequest, "bad_request", "rate_limit must be greater than 0")
		return
	}

	ak, err := h.store.Create(r.Context(), req.AppName, req.Origins, req.RateLimit)
	if err != nil {
		slog.Error("failed to create API key", "err", err)
		writeAdminHandlerError(w, http.StatusInternalServerError, "internal_error", "Failed to create API key")
		return
	}

	keyLabel := ak.Key
	if len(keyLabel) > 12 {
		keyLabel = keyLabel[:12] + "..."
	}
	slog.Info("api key created", "key", keyLabel, "app_name", ak.AppName)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(ak)
}

// ListKeys handles GET /admin/keys.
//
// @Summary      List all API keys
// @Description  Returns all provisioned API keys with metadata.
// @Tags         admin
// @Produce      json
// @Success      200  {array}   apikey.APIKey
// @Failure      500  {object}  model.ErrorResponse  "Internal error"
// @Security     AdminAuth
// @Router       /admin/keys [get]
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
//
// @Summary      Get API key details
// @Description  Returns metadata for a specific API key.
// @Tags         admin
// @Produce      json
// @Param        key  path  string  true  "API key (e.g. pk_abc123)"
// @Success      200  {object}  apikey.APIKey
// @Failure      404  {object}  model.ErrorResponse  "Key not found"
// @Failure      500  {object}  model.ErrorResponse  "Internal error"
// @Security     AdminAuth
// @Router       /admin/keys/{key} [get]
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
//
// @Summary      Deactivate API key
// @Description  Marks an API key as inactive. The key remains retrievable but will be rejected by auth middleware.
// @Tags         admin
// @Produce      json
// @Param        key  path  string  true  "API key to deactivate"
// @Success      200  {object}  map[string]string  "status: deactivated"
// @Failure      500  {object}  model.ErrorResponse  "Internal error"
// @Security     AdminAuth
// @Router       /admin/keys/{key} [delete]
func (h *AdminHandler) DeactivateKey(w http.ResponseWriter, r *http.Request) {
	keyStr := chi.URLParam(r, "key")

	if err := h.store.Deactivate(r.Context(), keyStr); err != nil {
		slog.Error("failed to deactivate API key", "err", err, "key", keyStr)
		writeAdminHandlerError(w, http.StatusInternalServerError, "internal_error", "Failed to deactivate API key")
		return
	}

	redacted := keyStr
	if len(redacted) > 12 {
		redacted = redacted[:12] + "..."
	}
	slog.Info("api key deactivated", "key", redacted)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "deactivated"})
}

// RotateKey handles POST /admin/keys/{key}/rotate.
//
// @Summary      Rotate API key
// @Description  Creates a new key with the same metadata and sets a grace period expiration on the old key. Both keys are valid during the grace period.
// @Tags         admin
// @Accept       json
// @Produce      json
// @Param        key   path  string            true  "API key to rotate"
// @Param        body  body  rotateKeyRequest  true  "Grace period for old key"
// @Success      200  {object}  rotateKeyResponse
// @Failure      400  {object}  model.ErrorResponse  "Invalid request or grace period"
// @Failure      500  {object}  model.ErrorResponse  "Internal error"
// @Security     AdminAuth
// @Router       /admin/keys/{key}/rotate [post]
func (h *AdminHandler) RotateKey(w http.ResponseWriter, r *http.Request) {
	oldKeyStr := chi.URLParam(r, "key")

	var req rotateKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAdminHandlerError(w, http.StatusBadRequest, "invalid_request", "Invalid JSON body")
		return
	}

	gracePeriod, err := time.ParseDuration(req.GracePeriod)
	if err != nil {
		writeAdminHandlerError(w, http.StatusBadRequest, "bad_request", "Invalid grace_period duration")
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

	oldRedacted := oldKeyStr
	if len(oldRedacted) > 12 {
		oldRedacted = oldRedacted[:12] + "..."
	}
	newRedacted := newKey.Key
	if len(newRedacted) > 12 {
		newRedacted = newRedacted[:12] + "..."
	}
	slog.Info("api key rotated", "old_key", oldRedacted, "new_key", newRedacted)
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
