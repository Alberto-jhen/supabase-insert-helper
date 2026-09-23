package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// ErrorResponse is the standard JSON body returned for HTTP errors.
type ErrorResponse struct {
	Error string `json:"error"`
}

// WriteJSON writes a JSON response with the provided status code.
// If serialization fails, the error is logged with slog.
func WriteJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		slog.Error("failed to encode JSON response", "error", err)
	}
}

// WriteError returns a standardized error response and logs it with slog.
func WriteError(w http.ResponseWriter, status int, message string) {
	slog.Error("http error", "status", status, "message", message)
	WriteJSON(w, status, ErrorResponse{Error: message})
}
