package handlers

import (
	"context"
	"log"
	"net/http"

	"pizza-backend/internal/repo"
	"pizza-backend/internal/telegram"
)

type OrdersDeps struct {
	Orders   *repo.Orders
	Telegram *telegram.Client
}

func CreateOrder(d *OrdersDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Name          string  `json:"name"`
			Phone         string  `json:"phone"`
			Address       string  `json:"address"`
			Zone          string  `json:"zone"`
			Comment       string  `json:"comment"`
			ReceiveMethod string  `json:"receiveMethod"`
			PayMethod     string  `json:"payMethod"`
			DeliveryTime  string  `json:"deliveryTime"`
			Items         []repo.OrderItem `json:"items"`
			Total         float64 `json:"total"`
			Delivery      float64 `json:"delivery"`
			PaymentID     string  `json:"paymentId"`
		}
		if err := decodeJSON(w, r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if req.Name == "" || req.Phone == "" || len(req.Items) == 0 {
			writeError(w, http.StatusBadRequest, "missing required fields")
			return
		}
		if req.ReceiveMethod == "" {
			req.ReceiveMethod = "delivery"
		}
		if req.PayMethod == "" {
			req.PayMethod = "cash"
		}
		if req.DeliveryTime == "" {
			req.DeliveryTime = "asap"
		}

		o := &repo.Order{
			CustomerName:  req.Name,
			CustomerPhone: req.Phone,
			Address:       req.Address,
			Zone:          req.Zone,
			Comment:       req.Comment,
			ReceiveMethod: req.ReceiveMethod,
			PayMethod:     req.PayMethod,
			DeliveryTime:  req.DeliveryTime,
			Items:         req.Items,
			Total:         req.Total,
			Delivery:      req.Delivery,
			PaymentID:     req.PaymentID,
		}
		inserted, err := d.Orders.Create(r.Context(), o)
		if err != nil {
			log.Printf("order create: %v", err)
			writeError(w, http.StatusInternalServerError, "failed to create order")
			return
		}

		if inserted {
			go func(o repo.Order) {
				ctx, cancel := context.WithTimeout(context.Background(), 10*1e9)
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
					log.Printf("telegram notify failed: %v", err)
				}
			}(*o)
		}

		writeJSON(w, http.StatusCreated, map[string]any{
			"id":     o.ID,
			"number": o.Number,
			"status": o.Status,
			"eta":    o.EtaMinutes,
		})
	}
}
