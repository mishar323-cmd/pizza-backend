package sms

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const baseURL = "https://sms.ru"

// ErrDisabled is returned when SMS_RU_API_ID is not configured. Callers can
// inspect this with errors.Is to fall back to console-only debug delivery
// during local dev.
var ErrDisabled = errors.New("sms.ru not configured")

type Client struct {
	apiID  string
	sender string // optional named sender; "" means use sms.ru default
	http   *http.Client
}

func NewClient(apiID, sender string) *Client {
	return &Client{
		apiID:  apiID,
		sender: sender,
		http:   &http.Client{Timeout: 10 * time.Second},
	}
}

// Configured reports whether the client has an API id and can hit sms.ru.
func (c *Client) Configured() bool { return c != nil && c.apiID != "" }

// SendResult is a minimal subset of sms.ru's send response.
type SendResult struct {
	Status     string  `json:"status"`
	StatusCode int     `json:"status_code"`
	SMSID      string  `json:"sms_id"`
	Balance    float64 `json:"balance"`
}

// Send delivers a plain SMS. Returns the sms_id on success.
//
// API: https://sms.ru/?panel=api/method&type=send
func (c *Client) Send(ctx context.Context, phone, text string) (*SendResult, error) {
	if !c.Configured() {
		return nil, ErrDisabled
	}
	p := normalizePhone(phone)
	if p == "" {
		return nil, fmt.Errorf("invalid phone: %q", phone)
	}

	q := url.Values{}
	q.Set("api_id", c.apiID)
	q.Set("to", p)
	q.Set("msg", text)
	q.Set("json", "1")
	if c.sender != "" {
		q.Set("from", c.sender)
	}

	var raw struct {
		Status     string  `json:"status"`
		StatusCode int     `json:"status_code"`
		StatusText string  `json:"status_text"`
		SMS        map[string]struct {
			Status     string `json:"status"`
			StatusCode int    `json:"status_code"`
			StatusText string `json:"status_text"`
			SMSID      string `json:"sms_id"`
		} `json:"sms"`
		Balance float64 `json:"balance"`
	}
	if err := c.get(ctx, "/sms/send", q, &raw); err != nil {
		return nil, err
	}
	if raw.Status != "OK" {
		return nil, fmt.Errorf("sms.ru: %s (code %d): %s", raw.Status, raw.StatusCode, raw.StatusText)
	}
	out := &SendResult{
		Status:     raw.Status,
		StatusCode: raw.StatusCode,
		Balance:    raw.Balance,
	}
	for _, v := range raw.SMS {
		if v.Status != "OK" {
			return nil, fmt.Errorf("sms.ru send to %s: %s (code %d): %s", p, v.Status, v.StatusCode, v.StatusText)
		}
		out.SMSID = v.SMSID
		break
	}
	return out, nil
}

// CallResult is the response to a flash-call OTP request.
type CallResult struct {
	Status     string  `json:"status"`
	StatusCode int     `json:"status_code"`
	Code       string  `json:"code"`    // last 4 digits of the calling number — this IS the OTP code
	CallID     string  `json:"call_id"`
	Cost       float64 `json:"cost"`
	Balance    float64 `json:"balance"`
}

// Call performs a flash-call (звонок-сброс): sms.ru rings the user's phone from
// a unique number; the last 4 digits of that number are the OTP code. The user
// reads them off the incoming call screen, does NOT answer. ~3x cheaper than
// SMS in Russia.
//
// API: https://sms.ru/?panel=api/method&type=callcheck
func (c *Client) Call(ctx context.Context, phone string) (*CallResult, error) {
	if !c.Configured() {
		return nil, ErrDisabled
	}
	p := normalizePhone(phone)
	if p == "" {
		return nil, fmt.Errorf("invalid phone: %q", phone)
	}

	q := url.Values{}
	q.Set("api_id", c.apiID)
	q.Set("phone", p)

	var raw struct {
		Status     string  `json:"status"`
		StatusCode int     `json:"status_code"`
		StatusText string  `json:"status_text"`
		Code       string  `json:"code"`
		CallID     string  `json:"call_id"`
		Cost       string  `json:"cost"`
		Balance    float64 `json:"balance"`
	}
	if err := c.get(ctx, "/code/call", q, &raw); err != nil {
		return nil, err
	}
	if raw.Status != "OK" {
		return nil, fmt.Errorf("sms.ru call: %s (code %d): %s", raw.Status, raw.StatusCode, raw.StatusText)
	}
	return &CallResult{
		Status:     raw.Status,
		StatusCode: raw.StatusCode,
		Code:       raw.Code,
		CallID:     raw.CallID,
		Balance:    raw.Balance,
	}, nil
}

func (c *Client) get(ctx context.Context, path string, q url.Values, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+path+"?"+q.Encode(), nil)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("sms.ru %s: %w", path, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	if err != nil {
		return err
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("sms.ru %s: http %d: %s", path, resp.StatusCode, string(body))
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("sms.ru %s: decode: %w (body=%q)", path, err, string(body))
	}
	return nil
}

var digitsOnly = regexp.MustCompile(`\D+`)

// normalizePhone returns a +7XXXXXXXXXX number with country code, or "" if the
// input doesn't look like a Russian mobile.
func normalizePhone(p string) string {
	p = digitsOnly.ReplaceAllString(p, "")
	if strings.HasPrefix(p, "8") && len(p) == 11 {
		p = "7" + p[1:]
	}
	if len(p) == 10 && (strings.HasPrefix(p, "9")) {
		p = "7" + p
	}
	if len(p) != 11 || !strings.HasPrefix(p, "7") {
		return ""
	}
	return p
}
