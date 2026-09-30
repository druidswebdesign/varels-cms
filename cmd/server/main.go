package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/druidswebdesign/varels-cms/internal/auth"
	"github.com/druidswebdesign/varels-cms/internal/db"
	"github.com/druidswebdesign/varels-cms/internal/handlers/admin"
	"github.com/druidswebdesign/varels-cms/internal/handlers/analytics"
	"github.com/druidswebdesign/varels-cms/internal/handlers/authweb"
	"github.com/druidswebdesign/varels-cms/internal/handlers/catalog"
	"github.com/druidswebdesign/varels-cms/internal/handlers/common"
	"github.com/druidswebdesign/varels-cms/internal/handlers/finance"
	"github.com/druidswebdesign/varels-cms/internal/handlers/inventory"
	"github.com/druidswebdesign/varels-cms/internal/handlers/middleware"
	"github.com/druidswebdesign/varels-cms/internal/handlers/sales"
)

// config holds runtime configuration loaded from the environment.
type config struct {
	Port          string
	DBPath        string
	Env           string
	SessionSecret string

	// Google OAuth is disabled; email + bcrypt is the only sign-in path.
	// GoogleClientID     string
	// GoogleClientSecret string
	// GoogleRedirectURL  string

	// InitialOwnerEmail string
	AdminEmail    string
	AdminPassword string

	BackupDir       string
	BackupInterval  time.Duration
	BackupRetention int
}

func loadConfig() config {
	return config{
		Port:               getenv("PORT", "8080"),
		DBPath:             getenv("DB_PATH", "./data/app.db"),
		Env:                getenv("APP_ENV", "development"),
		SessionSecret:      os.Getenv("SESSION_SECRET"),
		AdminEmail:         os.Getenv("ADMIN_EMAIL"),
		AdminPassword:      os.Getenv("ADMIN_PASSWORD"),

		BackupDir:       getenv("BACKUP_DIR", filepath.Join(filepath.Dir(getenv("DB_PATH", "./data/app.db")), "backups")),
		BackupInterval:  getenvDuration("BACKUP_INTERVAL", 24*time.Hour),
		BackupRetention: getenvInt("BACKUP_RETENTION", 7),
	}
}

// getenvDuration parses a duration like "12h" and falls back on error.
func getenvDuration(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		log.Printf("config: invalid %s=%q, using %s", key, v, fallback)
		return fallback
	}
	return d
}

// getenvInt parses an integer and falls back on error.
func getenvInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		log.Printf("config: invalid %s=%q, using %d", key, v, fallback)
		return fallback
	}
	return n
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func main() {
	// Load .env before reading config so `cp .env.example .env` just works.
	// Real environment variables always take precedence.
	loadDotEnv(".env")

	cfg := loadConfig()
	secure := cfg.Env != "development"

	// Validate secrets before touching the database so a misconfigured
	// production deployment fails fast instead of seeding a weak account.
	if secure && cfg.AdminPassword != "" {
		if err := auth.ValidatePasswordStrength(cfg.AdminPassword); err != nil {
			log.Fatalf("server: %v", err)
		}
	}
	if secure && len(cfg.SessionSecret) < 32 {
		log.Fatalf("server: SESSION_SECRET must be at least 32 bytes in production")
	}

	sqldb, err := db.Open(cfg.DBPath)
	if err != nil {
		log.Fatalf("server: %v", err)
	}
	defer sqldb.Close()

	if err := db.Migrate(sqldb); err != nil {
		log.Fatalf("server: %v", err)
	}

	ctx := context.Background()
	if err := db.Seed(ctx, sqldb, db.SeedOptions{
		AdminEmail:    cfg.AdminEmail,
		AdminPassword: cfg.AdminPassword,
		Dev:           cfg.Env == "development",
	}); err != nil {
		log.Fatalf("server: %v", err)
	}

	db.StartBackupSchedule(ctx, sqldb, cfg.BackupDir, cfg.BackupInterval, cfg.BackupRetention)

	srv := common.NewServer(sqldb, cfg.DBPath, cfg.Env)
	srv.Sessions = auth.NewManager(srv.Q, 24*time.Hour, secure)
	srv.Limiter = auth.NewLoginLimiter(5, 15*time.Minute)
	// srv.OAuthEnabled = cfg.GoogleClientID != "" && cfg.GoogleClientSecret != ""

	// Periodically purge expired session rows so the sessions table does not
	// grow without bound.
	go func() {
		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := srv.Sessions.CleanupExpiredSessions(ctx); err != nil {
					log.Printf("session gc: %v", err)
				}
			}
		}
	}()

	secret := cfg.SessionSecret
	if len(secret) < 32 {
		log.Printf("warning: SESSION_SECRET is empty or too short; using an ephemeral secret (sessions will not survive a restart)")
		secret = auth.RandomSecret()
	}

	// Google OAuth is disabled; email + bcrypt is the only sign-in path. The
	// old Goth provider registration is kept below for reference.
	// var providers []goth.Provider
	// if srv.OAuthEnabled {
	// 	providers = append(providers, auth.GoogleProvider(cfg.GoogleClientID, cfg.GoogleClientSecret, cfg.GoogleRedirectURL))
	// } else {
	// 	log.Printf("warning: Google OAuth is not configured; /auth/google is disabled (use /admin-login)")
	// }
	// auth.SetupOAuth(secret, secure, providers...)

	httpSrv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           routes(srv),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("varels-cms listening on %s (env=%s)", httpSrv.Addr, cfg.Env)
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpSrv.Shutdown(shutdownCtx); err != nil {
		log.Printf("server: shutdown: %v", err)
	}
}

// routes mounts the HTTP handler tree (docs/routes.md).
func routes(s *common.Server) http.Handler {
	r := chi.NewRouter()

	authh := authweb.New(s)
	adminh := admin.New(s)
	catalogh := catalog.New(s)
	salesh := sales.New(s)
	inventoryh := inventory.New(s)
	financeh := finance.New(s)
	analyticsh := analytics.New(s)

	r.Use(middleware.RequestID)
	r.Use(middleware.Logger)
	r.Use(middleware.Recover)
	r.Use(middleware.SecurityHeaders)
	r.Use(middleware.CSRF)
	r.Use(s.Sessions.LoadAndSave)
	r.Use(s.FlashFromSession)

	r.Handle("/assets/*", http.StripPrefix("/assets/", http.FileServer(http.Dir("assets"))))
	r.Get("/healthz", adminh.HandleHealth)

	// Public auth routes (email + bcrypt only; Google OAuth removed).
	r.Get("/login", authh.HandleLoginPage)
	r.Post("/login", authh.HandleLogin)

	// Google OAuth sign-in disabled (ADR-0004). Kept for reference:
	// r.Get("/auth/google", authh.HandleGoogleBegin)
	// r.Get("/auth/google/callback", authh.HandleGoogleCallback)
	// r.Get("/auth/denied", authh.HandleAccessDenied)
	// r.Get("/admin-login", authh.HandleAdminLoginPage)
	// r.Post("/admin-login", authh.HandleAdminLogin)
	r.Post("/logout", authh.HandleLogout)

	r.Group(func(r chi.Router) {
		r.Use(s.Sessions.RequireLogin)

		r.Get("/", adminh.HandleIndex)
		r.Get("/dashboard", adminh.HandleDashboard)

		r.Get("/products", catalogh.HandleProductsList)
		r.Get("/products/search", catalogh.HandleProductsSearch)
		r.Get("/products/new", catalogh.HandleProductForm)
		r.Post("/products", catalogh.HandleProductCreate)
		r.Get("/products/{id}", catalogh.HandleProductDetail)
		r.Get("/products/{id}/edit", catalogh.HandleProductEdit)
		r.Post("/products/{id}", catalogh.HandleProductUpdate)
		r.Post("/products/{id}/archive", catalogh.HandleProductArchive)
		r.Post("/products/{id}/restore", catalogh.HandleProductRestore)
		r.Post("/products/{id}/collections", catalogh.HandleProductCollections)
		r.Post("/products/{id}/media", catalogh.HandleMediaCreate)
		r.Post("/media/{id}/delete", catalogh.HandleMediaDelete)

		// Collections.
		r.Get("/collections", catalogh.HandleCollectionsList)
		r.Post("/collections", catalogh.HandleCollectionCreate)
		r.Post("/collections/{id}/archive", catalogh.HandleCollectionArchive)

		r.Get("/products/{id}/variants", catalogh.HandleVariantTable)
		r.Post("/products/{id}/variants", catalogh.HandleVariantCreate)
		r.Get("/variants/{id}/edit", catalogh.HandleVariantForm)
		r.Post("/variants/{id}", catalogh.HandleVariantUpdate)
		r.Post("/variants/{id}/archive", catalogh.HandleVariantArchive)
		r.Post("/variants/{id}/threshold", catalogh.HandleVariantThreshold)

		// Customers.
		r.Get("/customers", salesh.HandleCustomersList)
		r.Post("/customers", salesh.HandleCustomerCreate)
		r.Get("/customers/{id}", salesh.HandleCustomerDetail)
		r.Post("/customers/{id}", salesh.HandleCustomerUpdate)

		// Stock holds / reservations.
		r.Get("/holds", inventoryh.HandleHoldsList)
		r.Post("/holds", inventoryh.HandleHoldCreate)
		r.Post("/holds/{id}/release", inventoryh.HandleHoldRelease)

		// Inventory: restock, adjustments, ledger.
		r.Get("/restocks", inventoryh.HandleRestocksList)
		r.Get("/variants/{id}/restock", inventoryh.HandleRestockForm)
		r.Post("/variants/{id}/restock", inventoryh.HandleRestock)
		r.Post("/variants/{id}/adjust", inventoryh.HandleAdjustStock)
		r.Get("/stock-movements", inventoryh.HandleStockMovements)
		r.Get("/dashboard/low-stock", inventoryh.HandleLowStockWidget)

		// Sales / POS / returns.
		r.Get("/sales", salesh.HandleSalesList)
		r.Get("/sales/filter", salesh.HandleSalesFilter)
		r.Get("/sales/new", salesh.HandleSaleForm)
		r.Post("/sales", salesh.HandleSaleCreate)
		r.Get("/sales/{id}", salesh.HandleSaleDetail)
		r.Post("/sales/{id}/void", salesh.HandleSaleVoid)
		r.Get("/sales/{id}/return", salesh.HandleReturnForm)
		r.Post("/sales/{id}/return", salesh.HandleReturnCreate)
		r.Post("/sales/{id}/payments", salesh.HandlePaymentCreate)

		// Exports (financial resources are gated inside the handler).
		r.Get("/export/{resource}", financeh.HandleExport)

		r.Group(func(r chi.Router) {
			r.Use(s.Sessions.RequireRole(auth.RoleAdmin))

			r.Get("/dashboard/profit-cards", analyticsh.HandleProfitCards)

			r.Get("/staff", adminh.HandleStaffList)
			r.Post("/staff/invite", adminh.HandleStaffInvite)
			r.Post("/staff/{id}/role", adminh.HandleStaffRole)
			r.Post("/staff/{id}/disable", adminh.HandleStaffDisable)
			r.Post("/staff/{id}/enable", adminh.HandleStaffEnable)
			r.Post("/staff/{id}/reset-password", adminh.HandleStaffResetPassword)

			// Suppliers & purchase orders (supplier terms are admin-only).
			r.Get("/suppliers", adminh.HandleSuppliersList)
			r.Post("/suppliers", adminh.HandleSupplierCreate)
			r.Post("/suppliers/{id}", adminh.HandleSupplierUpdate)
			r.Get("/purchase-orders", adminh.HandlePurchaseOrdersList)
			r.Post("/purchase-orders", adminh.HandlePurchaseOrderCreate)
			r.Get("/purchase-orders/{id}", adminh.HandlePurchaseOrderDetail)
			r.Post("/purchase-orders/{id}/items", adminh.HandlePurchaseOrderAddItem)
			r.Post("/purchase-orders/{id}/status", adminh.HandlePurchaseOrderStatus)
			r.Post("/purchase-orders/{id}/receive", adminh.HandlePurchaseOrderReceive)

			// Audit trail.
			r.Get("/audit", adminh.HandleAuditLog)

			r.Get("/analytics", analyticsh.HandleAnalytics)
			r.Get("/analytics/best-sellers", analyticsh.HandleBestSellers)
			r.Get("/analytics/margin", analyticsh.HandleMargin)
			r.Get("/analytics/chart", analyticsh.HandleSalesChart)
			r.Get("/analytics/pl", analyticsh.HandleProfitLoss)
			r.Get("/analytics/sizes", analyticsh.HandleSizeCurve)

			r.Get("/deadstock", inventoryh.HandleDeadstock)
			r.Get("/deadstock/table", inventoryh.HandleDeadstockTable)

			r.Get("/expenses", financeh.HandleExpensesList)
			r.Post("/expenses", financeh.HandleExpenseCreate)
			r.Get("/expense-categories", financeh.HandleExpenseCategories)
			r.Post("/expense-categories", financeh.HandleExpenseCategoryCreate)

			r.Post("/admin/backup", adminh.HandleBackup)
			r.Post("/admin/restore-drill", adminh.HandleRestoreDrill)
		})
	})

	return r
}
