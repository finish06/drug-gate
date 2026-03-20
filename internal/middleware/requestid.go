package middleware

import (
	"context"
	"crypto/rand"
	"fmt"
	"net/http"
)

type ctxKeyRequestID struct{}

const maxRequestIDLen = 128

// RequestID is middleware that generates or propagates an X-Request-ID header.
// If the client provides a non-empty X-Request-ID, it is used (truncated to
// 128 chars). Otherwise a new UUID v4 is generated. The ID is stored in the
// request context and set in the response header.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if id == "" {
			id = newUUID()
		}
		if len(id) > maxRequestIDLen {
			id = id[:maxRequestIDLen]
		}

		w.Header().Set("X-Request-ID", id)
		ctx := context.WithValue(r.Context(), ctxKeyRequestID{}, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequestIDFromContext extracts the request ID from the context.
// Returns empty string if not set.
func RequestIDFromContext(ctx context.Context) string {
	if id, ok := ctx.Value(ctxKeyRequestID{}).(string); ok {
		return id
	}
	return ""
}

// newUUID generates a UUID v4 using crypto/rand.
func newUUID() string {
	var buf [16]byte
	_, _ = rand.Read(buf[:])
	buf[6] = (buf[6] & 0x0f) | 0x40 // version 4
	buf[8] = (buf[8] & 0x3f) | 0x80 // variant 10
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		buf[0:4], buf[4:6], buf[6:8], buf[8:10], buf[10:16])
}
