package auth

import (
	"strings"
)

// safeRedirectPath returns path+query for same-origin redirects after sign-in.
// Rejects absolute URLs, scheme-relative URLs, and javascript:.
func safeRedirectPath(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	lower := strings.ToLower(raw)
	if strings.Contains(lower, "javascript:") || strings.Contains(lower, "data:") {
		return ""
	}
	if strings.Contains(raw, "://") || strings.HasPrefix(raw, "//") {
		return ""
	}
	if !strings.HasPrefix(raw, "/") {
		return ""
	}
	return raw
}
