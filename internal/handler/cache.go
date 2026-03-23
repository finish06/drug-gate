package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	_ "github.com/finish06/drug-gate/internal/model"
	"github.com/redis/go-redis/v9"
)

// CacheHandler provides HTTP handlers for cache management.
type CacheHandler struct {
	rdb *redis.Client
}

// NewCacheHandler creates a CacheHandler with the given Redis client.
func NewCacheHandler(rdb *redis.Client) *CacheHandler {
	return &CacheHandler{rdb: rdb}
}

type cacheClearResult struct {
	Status      string `json:"status"`
	KeysDeleted int    `json:"keys_deleted"`
}

// ClearCache handles DELETE /admin/cache.
//
// @Summary      Clear Redis cache
// @Description  Deletes all cache keys matching cache:*, or a subset matching cache:{prefix}* if the prefix query parameter is provided. Does not affect API key or rate limit data.
// @Tags         admin
// @Produce      json
// @Param        prefix  query  string  false  "Key prefix filter (e.g. rxnorm, drugnames)"
// @Success      200  {object}  cacheClearResult
// @Failure      502  {object}  model.ErrorResponse  "Redis unavailable"
// @Security     AdminAuth
// @Router       /admin/cache [delete]
func (h *CacheHandler) ClearCache(w http.ResponseWriter, r *http.Request) {
	prefix := r.URL.Query().Get("prefix")

	pattern := "cache:*"
	if prefix != "" {
		pattern = "cache:" + prefix + "*"
	}

	deleted, err := h.scanAndDelete(r.Context(), pattern)
	if err != nil {
		slog.Error("failed to clear cache", "err", err, "pattern", pattern)
		writeError(w, http.StatusBadGateway, "upstream_error", "Redis unavailable")
		return
	}

	slog.Info("cache cleared", "pattern", pattern, "keys_deleted", deleted)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(cacheClearResult{
		Status:      "ok",
		KeysDeleted: deleted,
	})
}

// scanAndDelete uses SCAN to find keys matching pattern and DEL them in batches.
func (h *CacheHandler) scanAndDelete(ctx context.Context, pattern string) (int, error) {
	var cursor uint64
	var deleted int

	for {
		keys, nextCursor, err := h.rdb.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return deleted, err
		}

		if len(keys) > 0 {
			n, err := h.rdb.Del(ctx, keys...).Result()
			if err != nil {
				return deleted, err
			}
			deleted += int(n)
		}

		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}

	return deleted, nil
}
