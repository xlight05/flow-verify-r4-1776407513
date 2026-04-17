package auth

import (
	"net/http"
	"strings"
)

// BearerToken extracts the bearer token from the Authorization header.
// Returns "" if header is missing, malformed, or not a Bearer scheme.
func BearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if h == "" {
		return ""
	}
	parts := strings.SplitN(strings.TrimSpace(h), " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}
