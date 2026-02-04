package helpers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
)

func ParseIntParam(r *http.Request, name string, defaultVal int) int {
	val := r.URL.Query().Get(name)
	if val == "" {
		return defaultVal
	}
	n, err := strconv.Atoi(val)
	if err != nil {
		return defaultVal
	}
	return n
}

func WriteJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		slog.Error("failed to encode response", "error", err)
	}
}

func WriteError(w http.ResponseWriter, status int, message string) {
	WriteJSON(w, status, map[string]string{"error": message})
}
