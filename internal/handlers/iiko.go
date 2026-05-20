package handlers

import (
	"log"
	"net/http"

	"pizza-backend/internal/iiko"
)

func IikoOrder(ik *iiko.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var raw map[string]any
		if err := decodeJSON(w, r, &raw); err != nil {
			log.Printf("iiko decode: %v", err)
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if err := ik.SendOrder(r.Context(), raw); err != nil {
			writeJSON(w, http.StatusNotImplemented, map[string]any{
				"error":    err.Error(),
				"received": raw,
			})
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	}
}
