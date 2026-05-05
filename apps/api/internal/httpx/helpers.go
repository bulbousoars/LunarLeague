package httpx

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Warn("write json", "err", err)
	}
}

// WriteJSON is the public helper for handler packages.
func WriteJSON(w http.ResponseWriter, status int, v any) { writeJSON(w, status, v) }

// ReadJSON decodes the body into v with a sane size cap.
func ReadJSON(r *http.Request, v any) error {
	r.Body = http.MaxBytesReader(nil, r.Body, 1<<20) // 1 MiB
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}

// WriteError writes a JSON error envelope.
func WriteError(w http.ResponseWriter, status int, err error) {
	msg := http.StatusText(status)
	if err != nil {
		msg = err.Error()
	}
	writeJSON(w, status, map[string]any{"error": msg})
}

// ContextUserID is the context key for the authenticated user id.
type ctxKey int

const userIDKey ctxKey = 1

func WithUserID(ctx context.Context, uid string) context.Context {
	return context.WithValue(ctx, userIDKey, uid)
}

func UserID(ctx context.Context) (string, error) {
	v, ok := ctx.Value(userIDKey).(string)
	if !ok || v == "" {
		return "", errors.New("not authenticated")
	}
	return v, nil
}
