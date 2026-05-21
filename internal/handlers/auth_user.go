package handlers

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"pizza-backend/internal/auth"
	"pizza-backend/internal/repo"
	"pizza-backend/internal/sms"

	"golang.org/x/crypto/bcrypt"
)

// OTP knobs. Tweak via constants here, not env, until proven necessary.
const (
	otpTTL              = 5 * time.Minute // how long a code stays valid
	otpResendCooldown   = 60 * time.Second // min interval between two requests for the same phone
	otpMaxAttempts      = 5                // max verify attempts before invalidation
	otpCodeLength       = 4                // call-check codes are 4 digits; SMS fallback matches
)

type UserAuthDeps struct {
	Users  *repo.Users
	OTPs   *repo.OTPs
	SMS    *sms.Client
	Secret []byte
}

// POST /api/auth/request-otp
// Body: {"phone":"+79xxxxxxxxx"}
// Resp: {"channel":"call"|"sms","resendIn":60,"expiresIn":300}
func RequestOTP(d *UserAuthDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Phone string `json:"phone"`
		}
		if err := decodeJSON(w, r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		phone := normalizeUserPhone(req.Phone)
		if phone == "" {
			writeError(w, http.StatusBadRequest, "invalid phone")
			return
		}

		// Cooldown: if a code was sent < otpResendCooldown ago, refuse.
		if cur, err := d.OTPs.Get(r.Context(), phone); err == nil && cur != nil {
			elapsed := time.Since(cur.SentAt)
			if elapsed < otpResendCooldown {
				writeJSON(w, http.StatusTooManyRequests, map[string]any{
					"error":    "wait before requesting another code",
					"resendIn": int((otpResendCooldown - elapsed).Seconds()) + 1,
				})
				return
			}
		}

		// Generate code + send.
		// Primary channel: flash-call (cheap, fast). sms.ru generates the code on its side.
		// Fallback: send SMS with our own code.
		var (
			code    string
			channel string
		)
		if d.SMS != nil && d.SMS.Configured() {
			res, err := d.SMS.Call(r.Context(), phone)
			if err == nil && res.Code != "" {
				code = res.Code
				channel = "call"
			} else {
				if err != nil {
					log.Printf("sms.ru call failed (%s): %v — falling back to SMS", phone, err)
				}
				generated, gerr := genNumericCode(otpCodeLength)
				if gerr != nil {
					writeError(w, http.StatusInternalServerError, "code gen failed")
					return
				}
				if _, serr := d.SMS.Send(r.Context(), phone, "Код для входа: "+generated); serr != nil {
					log.Printf("sms.ru sms send failed (%s): %v", phone, serr)
					writeError(w, http.StatusBadGateway, "sms provider unavailable")
					return
				}
				code = generated
				channel = "sms"
			}
		} else {
			// Dev mode — no provider configured. Log code instead.
			generated, gerr := genNumericCode(otpCodeLength)
			if gerr != nil {
				writeError(w, http.StatusInternalServerError, "code gen failed")
				return
			}
			code = generated
			channel = "dev"
			log.Printf("[DEV] OTP for %s: %s (configure SMS_RU_API_ID to send for real)", phone, code)
		}

		hash, err := bcrypt.GenerateFromPassword([]byte(code), bcrypt.MinCost)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "hash failed")
			return
		}
		if err := d.OTPs.Upsert(r.Context(), phone, string(hash), channel, otpTTL); err != nil {
			log.Printf("otp upsert failed (%s): %v", phone, err)
			writeError(w, http.StatusInternalServerError, "storage failed")
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"channel":   channel,
			"resendIn":  int(otpResendCooldown.Seconds()),
			"expiresIn": int(otpTTL.Seconds()),
		})
	}
}

// POST /api/auth/verify-otp
// Body: {"phone":"+79xxxxxxxxx","code":"1234","name":"Михаил"}
// Resp: {"token":"<jwt>","user":{...}}
//
// The optional `name` is saved on first successful login (or whenever user
// wants to update it through this endpoint).
func VerifyOTP(d *UserAuthDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Phone string `json:"phone"`
			Code  string `json:"code"`
			Name  string `json:"name"`
		}
		if err := decodeJSON(w, r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		phone := normalizeUserPhone(req.Phone)
		code := strings.TrimSpace(req.Code)
		if phone == "" || code == "" {
			writeError(w, http.StatusBadRequest, "phone and code required")
			return
		}

		row, err := d.OTPs.Get(r.Context(), phone)
		if err != nil {
			log.Printf("otp get failed (%s): %v", phone, err)
			writeError(w, http.StatusInternalServerError, "storage failed")
			return
		}
		if row == nil {
			writeError(w, http.StatusUnauthorized, "no active code, request a new one")
			return
		}
		if time.Now().After(row.ExpiresAt) {
			_ = d.OTPs.Delete(r.Context(), phone)
			writeError(w, http.StatusUnauthorized, "code expired")
			return
		}
		if row.Attempts >= otpMaxAttempts {
			_ = d.OTPs.Delete(r.Context(), phone)
			writeError(w, http.StatusUnauthorized, "too many attempts, request a new code")
			return
		}
		if bcrypt.CompareHashAndPassword([]byte(row.CodeHash), []byte(code)) != nil {
			_ = d.OTPs.IncrAttempts(r.Context(), phone)
			writeError(w, http.StatusUnauthorized, "invalid code")
			return
		}

		// Success — provision/load user and issue token.
		u, err := d.Users.GetOrCreate(r.Context(), phone)
		if err != nil {
			log.Printf("users get/create failed (%s): %v", phone, err)
			writeError(w, http.StatusInternalServerError, "user provisioning failed")
			return
		}
		name := strings.TrimSpace(req.Name)
		if name != "" && name != u.Name {
			if err := d.Users.UpdateName(r.Context(), u.ID, name); err == nil {
				u.Name = name
			}
		}
		_ = d.OTPs.Delete(r.Context(), phone)

		token, err := auth.IssueUserToken(d.Secret, auth.UserClaims{
			UserID: u.ID,
			Phone:  u.Phone,
			Name:   u.Name,
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "token issue failed")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"token": token,
			"user":  u,
		})
	}
}

// GET /api/auth/me — returns the logged-in user (RequireUser middleware).
func UserMe(d *UserAuthDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := auth.UserFromContext(r.Context())
		if claims == nil {
			writeError(w, http.StatusUnauthorized, "no session")
			return
		}
		u, err := d.Users.Get(r.Context(), claims.UserID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "load failed")
			return
		}
		if u == nil {
			writeError(w, http.StatusUnauthorized, "user not found")
			return
		}
		writeJSON(w, http.StatusOK, u)
	}
}

func normalizeUserPhone(s string) string {
	// Accept inputs like "+7 (915) 488-9419", "8 915 488 9419", "79154889419".
	digits := make([]byte, 0, 16)
	for i := 0; i < len(s); i++ {
		if s[i] >= '0' && s[i] <= '9' {
			digits = append(digits, s[i])
		}
	}
	d := string(digits)
	if strings.HasPrefix(d, "8") && len(d) == 11 {
		d = "7" + d[1:]
	}
	if len(d) == 10 && strings.HasPrefix(d, "9") {
		d = "7" + d
	}
	if len(d) != 11 || !strings.HasPrefix(d, "7") {
		return ""
	}
	return "+" + d
}

func genNumericCode(n int) (string, error) {
	const digits = "0123456789"
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	out := make([]byte, n)
	for i, b := range buf {
		out[i] = digits[int(b)%len(digits)]
	}
	return string(out), nil
}

// keep imports tidy
var _ = json.Marshal
var _ = fmt.Sprintf
