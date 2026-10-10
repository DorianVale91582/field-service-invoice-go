package invoice

import (
	"encoding/json"
	"errors"
	"net/http"
)

func Handler(service *Service) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /orders/invoice", func(w http.ResponseWriter, r *http.Request) {
		var order WorkOrder
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&order); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid work order"})
			return
		}
		result, err := service.CreateInvoice(r.Context(), order)
		if err != nil {
			status := http.StatusBadGateway
			var apiErr *APIError
			if errors.As(err, &apiErr) && apiErr.StatusCode >= 400 && apiErr.StatusCode < 500 {
				status = apiErr.StatusCode
			}
			writeJSON(w, status, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, result)
	})
	return mux
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
