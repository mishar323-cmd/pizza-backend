package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type Client struct {
	token  string
	chatID string
	http   *http.Client
}

func NewClient(token, chatID string) *Client {
	return &Client{
		token:  token,
		chatID: chatID,
		http:   &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *Client) Enabled() bool { return c.token != "" && c.chatID != "" }

type Item struct {
	Name  string  `json:"name"`
	Qty   int     `json:"qty"`
	Price float64 `json:"price"`
}

type Order struct {
	Name          string  `json:"name"`
	Phone         string  `json:"phone"`
	Address       string  `json:"address"`
	Comment       string  `json:"comment"`
	ReceiveMethod string  `json:"receiveMethod"`
	PayMethod     string  `json:"payMethod"`
	DeliveryTime  string  `json:"deliveryTime"`
	Items         []Item  `json:"items"`
	Total         float64 `json:"total"`
}

func (c *Client) SendOrderNotification(ctx context.Context, o Order) error {
	if !c.Enabled() {
		return nil
	}

	method := "🚴 Доставка"
	if o.ReceiveMethod == "pickup" {
		method = "🏃 Самовывоз"
	}
	pay := "💵 При получении"
	if o.PayMethod == "online" {
		pay = "💳 Онлайн (оплачено)"
	}
	timeLabel := "Ближайшее время"
	if o.DeliveryTime != "" && o.DeliveryTime != "asap" {
		timeLabel = "К " + o.DeliveryTime
	}

	var itemsB strings.Builder
	for _, it := range o.Items {
		fmt.Fprintf(&itemsB, "  • %s ×%d — %.0f ₽\n", it.Name, it.Qty, it.Price*float64(it.Qty))
	}

	orderID := strings.ToUpper(strconv.FormatInt(time.Now().UnixMilli(), 36))

	var lines []string
	lines = append(lines, fmt.Sprintf("🍕 *Новый заказ #%s*", orderID))
	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("👤 %s | %s", o.Name, o.Phone))
	lines = append(lines, fmt.Sprintf("%s | %s", method, pay))
	lines = append(lines, fmt.Sprintf("⏰ %s", timeLabel))
	if o.Address != "" {
		lines = append(lines, fmt.Sprintf("📍 %s", o.Address))
	}
	if o.Comment != "" {
		lines = append(lines, fmt.Sprintf("💬 %s", o.Comment))
	}
	lines = append(lines, "")
	lines = append(lines, "*Состав:*")
	lines = append(lines, strings.TrimRight(itemsB.String(), "\n"))
	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("*Итого: %.0f ₽*", o.Total))

	text := strings.Join(lines, "\n")

	payload := map[string]any{
		"chat_id":    c.chatID,
		"text":       text,
		"parse_mode": "Markdown",
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", c.token)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
		return fmt.Errorf("telegram status %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}
