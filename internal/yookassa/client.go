package yookassa

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const baseURL = "https://api.yookassa.ru/v3"

type Client struct {
	shopID string
	secret string
	http   *http.Client
}

func NewClient(shopID, secret string) *Client {
	return &Client{
		shopID: shopID,
		secret: secret,
		http:   &http.Client{Timeout: 15 * time.Second},
	}
}

type Item struct {
	Name  string  `json:"name"`
	Qty   int     `json:"qty"`
	Price float64 `json:"price"`
}

type CreatePaymentRequest struct {
	Amount      float64           `json:"amount"`
	Description string            `json:"description"`
	ReturnURL   string            `json:"returnUrl"`
	Phone       string            `json:"phone"`
	Items       []Item            `json:"items"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

func (c *Client) CreatePayment(ctx context.Context, req CreatePaymentRequest) ([]byte, int, error) {
	receiptItems := make([]map[string]any, 0, len(req.Items)+1)
	var itemsSum float64
	for _, it := range req.Items {
		receiptItems = append(receiptItems, map[string]any{
			"description":     truncate(it.Name, 128),
			"quantity":        strconv.Itoa(it.Qty),
			"amount":          map[string]string{"value": fmt.Sprintf("%.2f", it.Price), "currency": "RUB"},
			"vat_code":        1,
			"payment_mode":    "full_payment",
			"payment_subject": "commodity",
		})
		itemsSum += it.Price * float64(it.Qty)
	}
	if delivery := req.Amount - itemsSum; delivery > 0 {
		receiptItems = append(receiptItems, map[string]any{
			"description":     "Доставка",
			"quantity":        "1",
			"amount":          map[string]string{"value": fmt.Sprintf("%.2f", delivery), "currency": "RUB"},
			"vat_code":        1,
			"payment_mode":    "full_payment",
			"payment_subject": "service",
		})
	}

	payload := map[string]any{
		"amount":       map[string]string{"value": fmt.Sprintf("%.2f", req.Amount), "currency": "RUB"},
		"capture":      true,
		"confirmation": map[string]string{"type": "redirect", "return_url": req.ReturnURL},
		"description":  req.Description,
		"receipt": map[string]any{
			"customer": map[string]string{"phone": normalizePhone(req.Phone)},
			"items":    receiptItems,
		},
	}
	if len(req.Metadata) > 0 {
		// YooKassa caps each value at 512 chars; trim defensively.
		md := make(map[string]string, len(req.Metadata))
		for k, v := range req.Metadata {
			if len(v) > 500 {
				v = v[:500]
			}
			md[k] = v
		}
		payload["metadata"] = md
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, 0, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/payments", bytes.NewReader(body))
	if err != nil {
		return nil, 0, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Idempotence-Key", idempotenceKey())
	httpReq.SetBasicAuth(c.shopID, c.secret)

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	return data, resp.StatusCode, err
}

func idempotenceKey() string {
	return strconv.FormatInt(time.Now().UnixNano(), 36) + strconv.FormatInt(rand.Int63(), 36)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

var digitsOnly = regexp.MustCompile(`\D+`)

func normalizePhone(p string) string {
	p = digitsOnly.ReplaceAllString(p, "")
	if strings.HasPrefix(p, "8") {
		p = "7" + p[1:]
	}
	return p
}
