package handler

import (
	"math"
	"net/http"
	"strconv"

	"github.com/finish06/drug-gate/internal/model"
)

// paginationParams holds parsed pagination query parameters.
type paginationParams struct {
	Page  int
	Limit int
}

// parsePagination extracts page and limit from query params with defaults and clamping.
func parsePagination(r *http.Request, defaultLimit, maxLimit int) paginationParams {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit < 1 {
		limit = defaultLimit
	}
	if limit > maxLimit {
		limit = maxLimit
	}

	return paginationParams{Page: page, Limit: limit}
}

// paginateSlice applies pagination to a slice and returns the page items plus metadata.
func paginateSlice[T any](items []T, p paginationParams) ([]T, model.Pagination) {
	total := len(items)
	totalPages := int(math.Ceil(float64(total) / float64(p.Limit)))
	if totalPages < 1 {
		totalPages = 1
	}

	start := (p.Page - 1) * p.Limit
	if start >= total {
		return []T{}, model.Pagination{
			Page:       p.Page,
			Limit:      p.Limit,
			Total:      total,
			TotalPages: totalPages,
		}
	}

	end := start + p.Limit
	if end > total {
		end = total
	}

	return items[start:end], model.Pagination{
		Page:       p.Page,
		Limit:      p.Limit,
		Total:      total,
		TotalPages: totalPages,
	}
}
