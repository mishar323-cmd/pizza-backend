package config

import (
	"crypto/rand"
	"encoding/base64"
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	YooKassaShopID string
	YooKassaSecret string
	TGBotToken     string
	TGChatID       string
	SMSRuAPIID     string
	SMSRuSender    string
	Port           string
	AllowOrigin    string
	DatabaseURL    string
	JWTSecret      []byte
	SeedAdminLogin string
	SeedAdminPass  string
	SeedAdminName  string
}

func Load() *Config {
	_ = godotenv.Load()
	_ = godotenv.Load("../.env")

	cfg := &Config{
		YooKassaShopID: os.Getenv("YOOKASSA_SHOP_ID"),
		YooKassaSecret: os.Getenv("YOOKASSA_SECRET"),
		TGBotToken:     os.Getenv("TG_BOT_TOKEN"),
		TGChatID:       os.Getenv("TG_CHAT_ID"),
		SMSRuAPIID:     os.Getenv("SMS_RU_API_ID"),
		SMSRuSender:    os.Getenv("SMS_RU_SENDER"),
		Port:           getenvDefault("PORT", "8080"),
		AllowOrigin:    os.Getenv("ALLOW_ORIGIN"),
		DatabaseURL:    os.Getenv("DATABASE_URL"),
		SeedAdminLogin: getenvDefault("ADMIN_LOGIN", "admin"),
		SeedAdminPass:  os.Getenv("ADMIN_PASSWORD"),
		SeedAdminName:  getenvDefault("ADMIN_NAME", "Администратор"),
	}

	if cfg.DatabaseURL == "" {
		log.Fatal("DATABASE_URL is required")
	}
	if cfg.YooKassaShopID == "" || cfg.YooKassaSecret == "" {
		log.Fatal("YOOKASSA_SHOP_ID and YOOKASSA_SECRET are required")
	}
	if cfg.TGBotToken == "" || cfg.TGChatID == "" {
		log.Println("WARN: TG_BOT_TOKEN/TG_CHAT_ID not set, Telegram notifications disabled")
	}
	if cfg.SMSRuAPIID == "" {
		log.Println("WARN: SMS_RU_API_ID not set, SMS sending + phone OTP disabled")
	}

	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		buf := make([]byte, 32)
		if _, err := rand.Read(buf); err != nil {
			log.Fatalf("generate jwt secret: %v", err)
		}
		secret = base64.StdEncoding.EncodeToString(buf)
		log.Println("WARN: JWT_SECRET not set, generated random ephemeral secret (sessions reset on restart)")
	}
	cfg.JWTSecret = []byte(secret)

	if cfg.SeedAdminPass == "" {
		log.Println("WARN: ADMIN_PASSWORD not set — no default admin will be seeded")
	}
	return cfg
}

func getenvDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
