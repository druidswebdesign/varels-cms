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

	"github.com/go-chi/chi/v5"
	"github.com/markbates/goth"

	"github.com/yourname/varels_cms/internal/auth"
	"github.com/yourname/varels_cms/internal/db"
	"github.com/yourname/varels_cms/internal/handlers"
)

// config holds runtime configuration loaded from the environment.
type config struct {
	Port          string
	DBPath        string
	Env           string
	SessionSecret string

	GoogleClientID     string
	GoogleClientSecret string
	GoogleRedirectURL  string

	InitialOwnerEmail string
	AdminEmail        string
	AdminPassword     string
}

func loadConfig() config {
	return config{
		Port:               getenv("PORT", "8080"),
		DBPath:             getenv("DB_PATH", "./data/app.db"),
		Env:                getenv("APP_ENV", "development"),
		SessionSecret:      os.Getenv("SESSION_SECRET"),
		GoogleClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		GoogleClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		GoogleRedirectURL:  getenv("GOOGLE_REDIRECT_URL", "http://localhost:8080/auth/google/callback"),
		InitialOwnerEmail:  os.Getenv("INITIAL_OWNER_EMAIL"),
		AdminEmail:         os.Getenv("ADMIN_EMAIL"),
		AdminPassword:      os.Getenv("ADMIN_PASSWORD"),
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func main() {
	cfg := loadConfig()
	secure := cfg.Env != "development"

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
		InitialOwnerEmail: cfg.InitialOwnerEmail,
		AdminEmail:        cfg.AdminEmail,
		AdminPassword:     cfg.AdminPassword,
		Dev:               cfg.Env == "development",
	}); err != nil {
		log.Fatalf("server: %v", err)
	}

	srv := handlers.NewServer(sqldb, cfg.DBPath, cfg.Env)
	srv.Sessions = auth.NewManager(srv.Q, 24*time.Hour, secure)
	srv.Limiter = auth.NewLoginLimiter(5, 15*time.Minute)
	srv.OAuthEnabled = cfg.GoogleClientID != "" && cfg.GoogleClientSecret != ""

	secret := cfg.SessionSecret
	if secret == "" {
		log.Printf("warning: SESSION_SECRET is empty; using an ephemeral secret (sessions will not survive a restart)")
		secret = auth.RandomSecret()
	}

	var providers []goth.Provider
	if srv.OAuthEnabled {
		providers = append(providers, auth.GoogleProvider(cfg.GoogleClientID, cfg.GoogleClientSecret, cfg.GoogleRedirectURL))
	} else {
		log.Printf("warning: Google OAuth is not configured; /auth/google is disabled (use /admin-login)")
	}
	auth.SetupOAuth(secret, secure, providers...)

	httpSrv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           routes(srv),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("varels_cms listening on %s (env=%s)", httpSrv.Addr, cfg.Env)
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
func routes(s *handlers.Server) http.Handler {
	r := chi.NewRouter()

	r.Use(handlers.RequestID)
	r.Use(handlers.Logger)
	r.Use(handlers.Recover)
	r.Use(handlers.CSRF)
	r.Use(s.Sessions.LoadAndSave)
	r.Use(s.FlashFromSession)

	r.Handle("/assets/*", http.StripPrefix("/assets/", http.FileServer(http.Dir("assets"))))
	r.Get("/healthz", s.HandleHealth)

	// Public auth routes.
	r.Get("/login", s.HandleLoginPage)
	r.Get("/auth/google", s.HandleGoogleBegin)
	r.Get("/auth/google/callback", s.HandleGoogleCallback)
	r.Get("/auth/denied", s.HandleAccessDenied)
	r.Get("/admin-login", s.HandleAdminLoginPage)
	r.Post("/admin-login", s.HandleAdminLogin)
	r.Post("/logout", s.HandleLogout)

	r.Group(func(r chi.Router) {
		r.Use(s.Sessions.RequireLogin)

		r.Get("/", s.HandleIndex)
		r.Get("/dashboard", s.HandleDashboard)

		r.Get("/products", s.HandleProductsList)
		r.Get("/products/search", s.HandleProductsSearch)
		r.Get("/products/new", s.HandleProductForm)
		r.Post("/products", s.HandleProductCreate)
		r.Get("/products/{id}", s.HandleProductDetail)
		r.Get("/products/{id}/edit", s.HandleProductEdit)
		r.Post("/products/{id}", s.HandleProductUpdate)
		r.Post("/products/{id}/archive", s.HandleProductArchive)
		r.Post("/products/{id}/restore", s.HandleProductRestore)
		r.Post("/products/{id}/collections", s.HandleProductCollections)
		r.Post("/products/{id}/media", s.HandleMediaCreate)
		r.Post("/media/{id}/delete", s.HandleMediaDelete)

		// Collections.
		r.Get("/collections", s.HandleCollectionsList)
		r.Post("/collections", s.HandleCollectionCreate)
		r.Post("/collections/{id}/archive", s.HandleCollectionArchive)

		r.Get("/products/{id}/variants", s.HandleVariantTable)
		r.Post("/products/{id}/variants", s.HandleVariantCreate)
		r.Get("/variants/{id}/edit", s.HandleVariantForm)
		r.Post("/variants/{id}", s.HandleVariantUpdate)
		r.Post("/variants/{id}/archive", s.HandleVariantArchive)
		r.Post("/variants/{id}/threshold", s.HandleVariantThreshold)

		// Customers.
		r.Get("/customers", s.HandleCustomersList)
		r.Post("/customers", s.HandleCustomerCreate)
		r.Get("/customers/{id}", s.HandleCustomerDetail)
		r.Post("/customers/{id}", s.HandleCustomerUpdate)

		// Stock holds / reservations.
		r.Get("/holds", s.HandleHoldsList)
		r.Post("/holds", s.HandleHoldCreate)
		r.Post("/holds/{id}/release", s.HandleHoldRelease)

		// Inventory: restock, adjustments, ledger.
		r.Get("/restocks", s.HandleRestocksList)
		r.Get("/variants/{id}/restock", s.HandleRestockForm)
		r.Post("/variants/{id}/restock", s.HandleRestock)
		r.Post("/variants/{id}/adjust", s.HandleAdjustStock)
		r.Get("/stock-movements", s.HandleStockMovements)
		r.Get("/dashboard/low-stock", s.HandleLowStockWidget)

		// Sales / POS / returns.
		r.Get("/sales", s.HandleSalesList)
		r.Get("/sales/filter", s.HandleSalesFilter)
		r.Get("/sales/new", s.HandleSaleForm)
		r.Post("/sales", s.HandleSaleCreate)
		r.Get("/sales/{id}", s.HandleSaleDetail)
		r.Post("/sales/{id}/void", s.HandleSaleVoid)
		r.Get("/sales/{id}/return", s.HandleReturnForm)
		r.Post("/sales/{id}/return", s.HandleReturnCreate)
		r.Post("/sales/{id}/payments", s.HandlePaymentCreate)

		// Exports (financial resources are gated inside the handler).
		r.Get("/export/{resource}", s.HandleExport)

		r.Group(func(r chi.Router) {
			r.Use(s.Sessions.RequireRole(auth.RoleAdmin))

			r.Get("/dashboard/profit-cards", s.HandleProfitCards)

			r.Get("/staff", s.HandleStaffList)
			r.Post("/staff/invite", s.HandleStaffInvite)
			r.Post("/staff/{id}/role", s.HandleStaffRole)
			r.Post("/staff/{id}/disable", s.HandleStaffDisable)
			r.Post("/staff/{id}/enable", s.HandleStaffEnable)
			r.Post("/staff/{id}/reset-password", s.HandleStaffResetPassword)

			// Suppliers & purchase orders (supplier terms are admin-only).
			r.Get("/suppliers", s.HandleSuppliersList)
			r.Post("/suppliers", s.HandleSupplierCreate)
			r.Post("/suppliers/{id}", s.HandleSupplierUpdate)
			r.Get("/purchase-orders", s.HandlePurchaseOrdersList)
			r.Post("/purchase-orders", s.HandlePurchaseOrderCreate)
			r.Get("/purchase-orders/{id}", s.HandlePurchaseOrderDetail)
			r.Post("/purchase-orders/{id}/items", s.HandlePurchaseOrderAddItem)
			r.Post("/purchase-orders/{id}/status", s.HandlePurchaseOrderStatus)
			r.Post("/purchase-orders/{id}/receive", s.HandlePurchaseOrderReceive)

			// Audit trail.
			r.Get("/audit", s.HandleAuditLog)

			r.Get("/analytics", s.HandleAnalytics)
			r.Get("/analytics/best-sellers", s.HandleBestSellers)
			r.Get("/analytics/margin", s.HandleMargin)
			r.Get("/analytics/chart", s.HandleSalesChart)
			r.Get("/analytics/pl", s.HandleProfitLoss)
			r.Get("/analytics/sizes", s.HandleSizeCurve)

			r.Get("/deadstock", s.HandleDeadstock)
			r.Get("/deadstock/table", s.HandleDeadstockTable)

			r.Get("/expenses", s.HandleExpensesList)
			r.Post("/expenses", s.HandleExpenseCreate)
			r.Get("/expense-categories", s.HandleExpenseCategories)
			r.Post("/expense-categories", s.HandleExpenseCategoryCreate)

			r.Post("/admin/backup", s.HandleBackup)
			r.Post("/admin/restore-drill", s.HandleRestoreDrill)
		})
	})

	return r
}
