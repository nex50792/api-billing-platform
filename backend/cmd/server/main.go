package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/joho/godotenv"
	"github.com/stripe/stripe-go/v85"

	"api-billing-platform/internal/handler"
	"api-billing-platform/internal/middleware"
	"api-billing-platform/internal/repository"
	"api-billing-platform/internal/service"
)

// 本番は JSON、開発は人間可読のテキスト形式でログ出力する。
func initLogger() {
	var h slog.Handler
	if os.Getenv("APP_ENV") == "production" {
		h = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})
	} else {
		h = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})
	}
	slog.SetDefault(slog.New(h))
}

func loadEnv() {
	if err := godotenv.Load(); err != nil {
		slog.Debug("no .env file, using OS environment", "error", err)
	}
}

func getenvDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func initDB() (*sql.DB, error) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = fmt.Sprintf(
			"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
			getenvDefault("DB_HOST", "localhost"),
			getenvDefault("DB_PORT", "5434"),
			getenvDefault("DB_USER", "postgres"),
			getenvDefault("DB_PASSWORD", "postgres"),
			getenvDefault("DB_NAME", "api_billing"),
		)
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := db.Ping(); err != nil {
		return nil, err
	}
	slog.Info("DB connection successful")
	return db, nil
}

func main() {
	initLogger()
	loadEnv()

	if key := os.Getenv("STRIPE_SECRET_KEY"); key != "" {
		stripe.Key = key
	}

	db, err := initDB()
	if err != nil {
		slog.Error("DB connection failed", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	allowedOrigins := []string{"http://localhost:3000", "http://localhost:5173", "http://localhost:5174"}
	if v := os.Getenv("ALLOWED_ORIGINS"); v != "" {
		allowedOrigins = strings.Split(v, ",")
	}

	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MB
			next.ServeHTTP(w, r)
		})
	})
	r.Use(middleware.Timeout(middleware.DefaultRequestTimeout))
	r.Use(middleware.SecurityHeaders)
	r.Use(middleware.CSRFProtect(allowedOrigins))
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   allowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	adminRepo := repository.NewAdminRepository(db)
	adminService := service.NewAdminService(adminRepo)
	adminHandler := handler.NewAdminHandlerWithDB(adminService, db)
	r.Post("/admin/login", adminHandler.Login)
	r.Post("/admin/logout", adminHandler.Logout)
	r.Get("/admin/me", adminHandler.Me)

	stripeWebhookHandler := handler.NewStripeWebhookHandler(db)
	stripeCheckoutHandler := handler.NewStripeCheckoutHandler(db)

	apikeyRepo := repository.NewAPIKeyRepository(db)
	apikeyService := service.NewAPIKeyService(apikeyRepo)
	apikeyHandler := handler.NewAPIKeyHandler(apikeyService, db)

	customerHandler := handler.NewCustomerHandler(db)
	customerHandler.Mailer = handler.NewSMTPMailer(db)

	adminAuth := middleware.NewAdminAuthMiddleware(adminService)
	providerSessionStore := repository.NewDBProviderSessionStore(db)
	providerAuth := middleware.NewProviderAuthMiddleware(providerSessionStore)
	customerSessionStore := repository.NewDBCustomerSessionStore(db)
	customerAuth := middleware.NewCustomerAuthMiddleware(customerSessionStore)

	r.Get("/health", handler.HealthHandler(db))

	r.With(customerAuth.Require()).Post("/apikeys", apikeyHandler.IssueAPIKey)
	r.With(customerAuth.Require()).Delete("/apikeys/{id}", apikeyHandler.DeleteAPIKey)
	r.With(customerAuth.Require()).Get("/customer/apikeys", apikeyHandler.GetCustomerAPIKeys)

	r.Post("/stripe/webhook", stripeWebhookHandler.Handle)
	r.With(customerAuth.Require()).Post("/customer/checkout", stripeCheckoutHandler.CreateCheckoutSession)
	r.With(customerAuth.Require()).Post("/customer/billing-portal", stripeCheckoutHandler.CreateBillingPortalSession)
	r.With(customerAuth.Require()).Get("/customer/subscription", stripeCheckoutHandler.GetSubscription)

	billingHandler := handler.NewBillingHandler(apikeyRepo, db)
	apikeyAuth := middleware.NewAPIKeyAuthMiddleware(apikeyRepo)
	r.With(apikeyAuth.Handler).Post("/billing/check", billingHandler.Check)

	r.Post("/customer/register", customerHandler.Signup)
	r.Post("/customer/verify", customerHandler.VerifyCode)
	r.Post("/customer/login", customerHandler.Login)
	r.Post("/customer/logout", customerHandler.Logout)
	r.Get("/customer/me", customerHandler.Me)
	r.Post("/customer/forgot-password", customerHandler.ForgotPassword)
	r.Post("/customer/reset-password", customerHandler.ResetPassword)
	r.With(customerAuth.Require()).Post("/customer/change-password", customerHandler.ChangePassword)

	r.Get("/provided-apis", handler.ProvidedAPIListHandler(db))
	r.With(providerAuth.Require()).Post("/provided-apis", handler.ProvidedAPICreateHandler(db))
	r.With(providerAuth.Require()).Put("/provided-apis/{id}", handler.ProvidedAPIUpdateHandler(db))
	r.With(providerAuth.Require()).Delete("/provided-apis/{id}", handler.ProvidedAPIDeleteHandler(db))

	r.With(adminAuth.Require()).Get("/admin/customers", adminHandler.GetCustomers)
	r.With(adminAuth.Require()).Get("/admin/email-logs", adminHandler.GetEmailLogs)

	r.With(adminAuth.Require()).Post("/admin/change-password", adminHandler.ChangePassword)

	providerHandler := handler.NewProviderHandler(db, handler.NewSMTPMailer(db))
	r.With(adminAuth.Require()).Get("/providers", providerHandler.GetProviders)
	r.With(adminAuth.Require()).Post("/providers", providerHandler.CreateProvider)
	r.With(adminAuth.Require()).Delete("/providers/{id}", providerHandler.DeleteProvider)
	r.Post("/provider/forgot-password", providerHandler.ForgotPassword)
	r.Post("/provider/validate-token", providerHandler.ValidatePasswordToken)
	r.Post("/provider/set-password", providerHandler.SetPassword)
	r.Get("/provider/me", providerHandler.Me)
	r.Post("/provider/login", providerHandler.Login)
	r.Post("/provider/logout", providerHandler.Logout)
	r.With(providerAuth.Require()).Get("/provider/apis", handler.ProvidedAPIProviderListHandler(db))
	r.With(providerAuth.Require()).Get("/provider/usage", handler.ProvidedAPIUsageHandler(db))
	r.With(providerAuth.Require()).Post("/provider/keys/verify", apikeyHandler.VerifyAPIKey)
	r.With(providerAuth.Require()).Post("/provider/change-password", providerHandler.ChangePassword)

	frontendURL := os.Getenv("FRONTEND_URL")
	if frontendURL == "" {
		frontendURL = "http://localhost:5173"
	}
	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		if r.URL != nil && len(r.URL.Path) >= 6 && r.URL.Path[:6] == "/admin" {
			target := frontendURL + r.URL.Path
			if r.URL.RawQuery != "" {
				target += "?" + r.URL.RawQuery
			}
			http.Redirect(w, r, target, http.StatusTemporaryRedirect)
			return
		}
		http.NotFound(w, r)
	})

	port := os.Getenv("SERVER_PORT")
	if port == "" {
		port = "8080"
	}

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	cleanupCtx, cleanupCancel := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				now := time.Now()
				for _, q := range []string{
					`DELETE FROM customer_sessions WHERE expires_at < $1`,
					`DELETE FROM provider_sessions WHERE expires_at < $1`,
					`DELETE FROM admin_sessions WHERE expires_at < $1`,
					`DELETE FROM customer_password_resets WHERE expires_at < $1 OR used_at IS NOT NULL`,
					`DELETE FROM customer_verification_codes WHERE expires_at < $1`,
					`UPDATE providers SET password_token_hash = NULL, password_token_expiry = NULL WHERE password_token_expiry < $1`,
					`DELETE FROM stripe_events WHERE processed_at < $1 - INTERVAL '90 days'`,
				} {
					if _, err := db.Exec(q, now); err != nil {
						slog.Warn("session cleanup failed", "error", err)
					}
				}
			case <-cleanupCtx.Done():
				return
			}
		}
	}()

	go func() {
		slog.Info("starting server", "port", port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	<-quit
	cleanupCancel()
	slog.Info("shutting down server")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("server forced shutdown", "error", err)
		os.Exit(1)
	}
	slog.Info("server stopped")
}
