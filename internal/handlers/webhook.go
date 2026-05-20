package handlers

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"pizza-backend/internal/repo"
	"pizza-backend/internal/telegram"
)

// YooKassa publishes webhook sender IPs at
// https://yookassa.ru/developers/using-api/webhooks#ip
// Update this list when YooKassa expands their ranges.
var yooKassaCIDRs = []string{
	"185.71.76.0/27",
	"185.71.77.0/27",
	"77.75.153.0/25",
	"77.75.154.128/25",
	"77.75.156.11/32",
	"77.75.156.35/32",
	"2a02:5180::/32",
}

type ykPayment struct {
	ID       string `json:"id"`
	Status   string `json:"status"`
	Paid     bool   `json:"paid"`
	Amount   struct {
		Value    string `json:"value"`
		Currency string `json:"currency"`
	} `json:"amount"`
	Metadata map[string]string `json:"metadata"`
	Receipt  struct {
		Customer struct {
			Phone string `json:"phone"`
		} `json:"customer"`
		Items []struct {
			Description    string `json:"description"`
			Quantity       string `json:"quantity"`
			Amount         struct {
				Value string `json:"value"`
			} `json:"amount"`
			PaymentSubject string `json:"payment_subject"`
		} `json:"items"`
	} `json:"receipt"`
}

type ykEvent struct {
	Event  string    `json:"event"`
	Object ykPayment `json:"object"`
}

// YooKassaWebhook handles payment lifecycle events. On payment.succeeded it
// idempotently persists the order (deduplicated by payment_id) and sends a
// Telegram notification. Returns 200 on every event YooKassa is allowed to
// resend — non-2xx triggers retry storms.
func YooKassaWebhook(d *OrdersDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !ipAllowed(clientIP(r)) {
			log.Printf("yookassa webhook: blocked IP %s", clientIP(r))
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}

		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			http.Error(w, "bad body", http.StatusBadRequest)
			return
		}

		var ev ykEvent
		if err := json.Unmarshal(body, &ev); err != nil {
			log.Printf("yookassa webhook: invalid JSON: %v", err)
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}

		// Ack non-success events so YooKassa doesn't retry.
		if ev.Event != "payment.succeeded" || !ev.Object.Paid {
			w.WriteHeader(http.StatusOK)
			return
		}

		o := buildOrderFromPayment(ev.Object)
		inserted, err := d.Orders.Create(r.Context(), o)
		if err != nil {
			log.Printf("yookassa webhook: order persist failed (payment %s): %v", ev.Object.ID, err)
			// Return 200 anyway — YooKassa retries are not worth the duplicate-storm risk.
			// Errors visible in app logs.
			w.WriteHeader(http.StatusOK)
			return
		}

		if inserted {
			go func(o repo.Order) {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				tgItems := make([]telegram.Item, 0, len(o.Items))
				for _, it := range o.Items {
					tgItems = append(tgItems, telegram.Item{Name: it.Name, Qty: it.Qty, Price: it.Price})
				}
				err := d.Telegram.SendOrderNotification(ctx, telegram.Order{
					Name: o.CustomerName, Phone: o.CustomerPhone, Address: o.Address,
					Comment: o.Comment, ReceiveMethod: o.ReceiveMethod, PayMethod: o.PayMethod,
					DeliveryTime: o.DeliveryTime, Items: tgItems, Total: o.Total,
				})
				if err != nil {
					log.Printf("telegram notify (webhook) failed: %v", err)
				}
			}(*o)
		}

		w.WriteHeader(http.StatusOK)
	}
}

func buildOrderFromPayment(p ykPayment) *repo.Order {
	md := p.Metadata
	o := &repo.Order{
		CustomerName:  strings.TrimSpace(md["name"]),
		CustomerPhone: strings.TrimSpace(firstNonEmpty(md["phone"], p.Receipt.Customer.Phone)),
		Address:       strings.TrimSpace(md["address"]),
		Zone:          strings.TrimSpace(md["zoneId"]),
		Comment:       strings.TrimSpace(md["comment"]),
		ReceiveMethod: strings.TrimSpace(firstNonEmpty(md["receiveMethod"], "delivery")),
		PayMethod:     "online",
		DeliveryTime:  strings.TrimSpace(firstNonEmpty(md["deliveryTime"], "asap")),
		PaymentID:     p.ID,
	}

	// Convert receipt items back into order items (excluding the delivery line).
	for _, it := range p.Receipt.Items {
		if it.PaymentSubject == "service" {
			if v, err := strconv.ParseFloat(it.Amount.Value, 64); err == nil {
				o.Delivery += v
			}
			continue
		}
		qty, _ := strconv.Atoi(it.Quantity)
		price, _ := strconv.ParseFloat(it.Amount.Value, 64)
		o.Items = append(o.Items, repo.OrderItem{Name: it.Description, Qty: qty, Price: price})
	}

	if v, err := strconv.ParseFloat(p.Amount.Value, 64); err == nil {
		o.Total = v
	}
	// If metadata claims a different delivery total, prefer it (more precise than
	// receipt rounding).
	if v, err := strconv.ParseFloat(md["delivery"], 64); err == nil && v >= 0 {
		o.Delivery = v
	}
	return o
}

func ipAllowed(ip string) bool {
	if ip == "" {
		// In dev without a proxy we may not get an IP. Allow only when no CIDR
		// list is configured (defensive: keep the default list non-empty).
		return false
	}
	addr := net.ParseIP(ip)
	if addr == nil {
		return false
	}
	for _, cidr := range yooKassaCIDRs {
		_, n, err := net.ParseCIDR(cidr)
		if err != nil {
			continue
		}
		if n.Contains(addr) {
			return true
		}
	}
	return false
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// First IP in the list is the original client.
		if i := strings.Index(xff, ","); i >= 0 {
			return strings.TrimSpace(xff[:i])
		}
		return strings.TrimSpace(xff)
	}
	if xr := r.Header.Get("X-Real-IP"); xr != "" {
		return strings.TrimSpace(xr)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
