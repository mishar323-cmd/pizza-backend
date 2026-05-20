package repo

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type OrderItem struct {
	Name  string  `json:"name"`
	Qty   int     `json:"qty"`
	Price float64 `json:"price"`
}

type Order struct {
	ID            int64       `json:"id"`
	Number        int         `json:"number"`
	CustomerName  string      `json:"customerName"`
	CustomerPhone string      `json:"customerPhone"`
	Address       string      `json:"address"`
	Zone          string      `json:"zone"`
	Comment       string      `json:"comment"`
	ReceiveMethod string      `json:"receiveMethod"`
	PayMethod     string      `json:"payMethod"`
	DeliveryTime  string      `json:"deliveryTime"`
	Items         []OrderItem `json:"items"`
	Total         float64     `json:"total"`
	Delivery      float64     `json:"delivery"`
	Status        string      `json:"status"`
	EtaMinutes    int         `json:"etaMinutes"`
	AssignedTo    string      `json:"assignedTo"`
	PaymentID     string      `json:"paymentId,omitempty"`
	CreatedAt     time.Time   `json:"createdAt"`
	UpdatedAt     time.Time   `json:"updatedAt"`
}

type Orders struct{ pool *pgxpool.Pool }

func NewOrders(pool *pgxpool.Pool) *Orders { return &Orders{pool: pool} }

// Create inserts an order. Returns (inserted, err). When payment_id is set and
// a row with that payment_id already exists, the existing row's fields are
// loaded into o and inserted=false (idempotent). For cash orders payment_id is
// empty so inserted is always true.
func (r *Orders) Create(ctx context.Context, o *Order) (bool, error) {
	itemsJSON, err := json.Marshal(o.Items)
	if err != nil {
		return false, err
	}
	err = r.pool.QueryRow(ctx, `
		INSERT INTO orders(number, customer_name, customer_phone, address, zone, comment,
			receive_method, pay_method, delivery_time, items, total, delivery, status, eta_minutes, assigned_to, payment_id)
		VALUES (nextval('order_number_seq'), $1, $2, $3, $4, $5, $6, $7, $8, $9::jsonb, $10, $11, $12, $13, $14, $15)
		ON CONFLICT (payment_id) WHERE payment_id IS NOT NULL AND payment_id <> ''
		DO NOTHING
		RETURNING id, number, status, eta_minutes, created_at, updated_at`,
		o.CustomerName, o.CustomerPhone, o.Address, o.Zone, o.Comment,
		o.ReceiveMethod, o.PayMethod, o.DeliveryTime, string(itemsJSON), o.Total,
		o.Delivery, ifEmpty(o.Status, "new"), ifZeroInt(o.EtaMinutes, 35), o.AssignedTo, o.PaymentID,
	).Scan(&o.ID, &o.Number, &o.Status, &o.EtaMinutes, &o.CreatedAt, &o.UpdatedAt)
	if err == nil {
		return true, nil
	}
	if err.Error() != "no rows in result set" {
		return false, err
	}
	// Conflict on payment_id — load the existing row instead.
	if o.PaymentID == "" {
		return false, err
	}
	loadErr := r.pool.QueryRow(ctx, `
		SELECT id, number, status, eta_minutes, created_at, updated_at
		FROM orders WHERE payment_id = $1`, o.PaymentID,
	).Scan(&o.ID, &o.Number, &o.Status, &o.EtaMinutes, &o.CreatedAt, &o.UpdatedAt)
	return false, loadErr
}

func (r *Orders) List(ctx context.Context, limit int) ([]Order, error) {
	if limit <= 0 {
		limit = 200
	}
	rows, err := r.pool.Query(ctx, `
		SELECT id, number, customer_name, customer_phone, COALESCE(address, ''), COALESCE(zone, ''), COALESCE(comment, ''),
			receive_method, pay_method, delivery_time, items, total, delivery, status, eta_minutes, COALESCE(assigned_to, ''), COALESCE(payment_id, ''), created_at, updated_at
		FROM orders ORDER BY created_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Order
	for rows.Next() {
		var o Order
		var itemsRaw []byte
		if err := rows.Scan(
			&o.ID, &o.Number, &o.CustomerName, &o.CustomerPhone, &o.Address, &o.Zone, &o.Comment,
			&o.ReceiveMethod, &o.PayMethod, &o.DeliveryTime, &itemsRaw, &o.Total, &o.Delivery,
			&o.Status, &o.EtaMinutes, &o.AssignedTo, &o.PaymentID, &o.CreatedAt, &o.UpdatedAt,
		); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(itemsRaw, &o.Items)
		out = append(out, o)
	}
	return out, rows.Err()
}

func (r *Orders) UpdateStatus(ctx context.Context, id int64, status string, assignedTo string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE orders SET status=$2, assigned_to=$3, updated_at=now() WHERE id=$1`,
		id, status, assignedTo,
	)
	return err
}

func ifEmpty(s, def string) string {
	if s == "" {
		return def
	}
	return s
}
func ifZeroInt(n, def int) int {
	if n == 0 {
		return def
	}
	return n
}
