package handlers

import (
	"log"
	"net/http"

	"pizza-backend/internal/telegram"
)

func NotifyTelegram(tg *telegram.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var order telegram.Order
		if err := decodeJSON(w, r, &order); err != nil {
			log.Printf("notify-telegram decode: %v", err)
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if err := tg.SendOrderNotification(r.Context(), order); err != nil {
			log.Printf("telegram error: %v", err)
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	}
}
