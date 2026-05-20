package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"pizza-backend/internal/auth"
	"pizza-backend/internal/config"
	"pizza-backend/internal/db"
	"pizza-backend/internal/handlers"
	"pizza-backend/internal/iiko"
	"pizza-backend/internal/repo"
	"pizza-backend/internal/telegram"
	"pizza-backend/internal/yookassa"
)

const (
	readHeaderTimeout = 5 * time.Second
	readTimeout       = 15 * time.Second
	writeTimeout      = 30 * time.Second
	idleTimeout       = 60 * time.Second
	maxHeaderBytes    = 1 << 14
	shutdownTimeout   = 15 * time.Second
)

func main() {
	cfg := config.Load()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool, err := db.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("db open: %v", err)
	}
	defer pool.Close()

	if err := db.Migrate(ctx, pool); err != nil {
		log.Fatalf("db migrate: %v", err)
	}

	admins := repo.NewAdmins(pool)
	orders := repo.NewOrders(pool)
	settings := repo.NewSettings(pool)

	seedAdmin(ctx, admins, cfg)

	yk := yookassa.NewClient(cfg.YooKassaShopID, cfg.YooKassaSecret)
	tg := telegram.NewClient(cfg.TGBotToken, cfg.TGChatID)
	ik := iiko.NewClient()

	adminDeps := &handlers.AdminDeps{
		Admins: admins, Orders: orders, Settings: settings, Secret: cfg.JWTSecret,
	}
	orderDeps := &handlers.OrdersDeps{Orders: orders, Telegram: tg}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", handlers.Health)
	mux.HandleFunc("POST /api/create-payment", handlers.CreatePayment(yk))
	mux.HandleFunc("POST /api/notify-telegram", handlers.NotifyTelegram(tg))
	mux.HandleFunc("POST /api/orders", handlers.CreateOrder(orderDeps))
	mux.HandleFunc("POST /api/iiko/order", handlers.IikoOrder(ik))

	mux.HandleFunc("POST /api/admin/login", handlers.AdminLogin(adminDeps))

	requireAdmin := auth.RequireAdmin(cfg.JWTSecret)
	adminMux := http.NewServeMux()
	adminMux.HandleFunc("GET /api/admin/me", handlers.AdminMe(adminDeps))
	adminMux.HandleFunc("GET /api/admin/orders", handlers.AdminOrdersList(adminDeps))
	adminMux.HandleFunc("PUT /api/admin/orders/{id}", handlers.AdminOrderUpdate(adminDeps))
	adminMux.HandleFunc("GET /api/admin/settings/{key}", handlers.AdminSettingsGet(adminDeps))
	adminMux.HandleFunc("PUT /api/admin/settings/{key}", handlers.AdminSettingsPut(adminDeps))
	mux.Handle("/api/admin/", requireAdmin(adminMux))

	handler := recoverPanic(withLogging(withCORS(cfg.AllowOrigin, mux)))

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           handler,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
		MaxHeaderBytes:    maxHeaderBytes,
	}

	go func() {
		log.Printf("pizza-backend listening on :%s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server error: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	log.Println("shutting down...")
	shutdownCtx, sCancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer sCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown error: %v", err)
	} else {
		log.Println("shutdown clean")
	}
}

func seedAdmin(ctx context.Context, admins *repo.Admins, cfg *config.Config) {
	if cfg.SeedAdminPass == "" {
		return
	}
	n, err := admins.Count(ctx)
	if err != nil {
		log.Printf("seed admin count: %v", err)
		return
	}
	if n > 0 {
		return
	}
	hash, err := auth.HashPassword(cfg.SeedAdminPass)
	if err != nil {
		log.Printf("seed admin hash: %v", err)
		return
	}
	if _, err := admins.Create(ctx, cfg.SeedAdminLogin, hash, cfg.SeedAdminName, "super"); err != nil {
		log.Printf("seed admin create: %v", err)
		return
	}
	log.Printf("seeded default admin: %s", cfg.SeedAdminLogin)
}

type statusRecorder struct {
	http.ResponseWriter
	status int
	wrote  bool
}

func (s *statusRecorder) WriteHeader(code int) {
	if !s.wrote {
		s.status = code
		s.wrote = true
	}
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusRecorder) Write(b []byte) (int, error) {
	if !s.wrote {
		s.status = http.StatusOK
		s.wrote = true
	}
	return s.ResponseWriter.Write(b)
}

func withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w}
		next.ServeHTTP(rec, r)
		if rec.status == 0 {
			rec.status = http.StatusOK
		}
		log.Printf("%s %s %d %s", r.Method, r.URL.Path, rec.status, time.Since(start))
	})
}

func recoverPanic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("PANIC %s %s: %v", r.Method, r.URL.Path, rec)
				http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func withCORS(origin string, next http.Handler) http.Handler {
	if origin == "" {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type,Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
