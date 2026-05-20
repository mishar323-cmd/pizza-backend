package handlers

import (
	"log"
	"net/http"

	"pizza-backend/internal/yookassa"
)

func CreatePayment(yk *yookassa.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req yookassa.CreatePaymentRequest
		if err := decodeJSON(w, r, &req); err != nil {
			log.Printf("create-payment decode: %v", err)
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if req.Amount <= 0 || len(req.Items) == 0 || req.ReturnURL == "" {
			writeError(w, http.StatusBadRequest, "invalid payment payload")
			return
		}

		data, status, err := yk.CreatePayment(r.Context(), req)
		if err != nil {
			log.Printf("yookassa error: %v", err)
			writeError(w, http.StatusBadGateway, "payment service unavailable")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write(data)
	}
}
