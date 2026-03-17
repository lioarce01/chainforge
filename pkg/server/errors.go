package server

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/lioarce01/chainforge/pkg/core"
)

// errorResponse is the JSON body for all error responses.
type errorResponse struct {
	Error string `json:"error"`
	Code  int    `json:"code"`
}

// writeError encodes an errorResponse as JSON with the given status code.
func writeError(w http.ResponseWriter, statusCode int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(errorResponse{Error: msg, Code: statusCode})
}

// statusFor maps core errors to HTTP status codes.
func statusFor(err error) int {
	if err == nil {
		return http.StatusOK
	}
	switch {
	case errors.Is(err, core.ErrNoProvider):
		return http.StatusServiceUnavailable
	case errors.Is(err, core.ErrNoModel):
		return http.StatusServiceUnavailable
	case errors.Is(err, core.ErrMaxIterations):
		return http.StatusUnprocessableEntity
	case errors.Is(err, core.ErrToolNotFound):
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}
