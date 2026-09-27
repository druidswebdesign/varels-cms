package handlers

import (
	"net/http"

	"github.com/yourname/varels_cms/internal/auth"
	"github.com/yourname/varels_cms/internal/db/sqlc"
	"github.com/yourname/varels_cms/internal/views/pages"
)

// HandleIndex redirects / to the dashboard.
func (s *Server) HandleIndex(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
}

// HandleDashboard renders GET /dashboard. Low-stock and P&L widgets load via
// HTMX from /dashboard/low-stock and /dashboard/profit-cards.
func (s *Server) HandleDashboard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	products, err := s.Q.ListProducts(ctx, sqlc.ListProductsParams{
		IsArchived: 0,
		CategoryID: nil,
		Offset:     0,
		Limit:      productListLimit,
	})
	if err != nil {
		serverError(w, err)
		return
	}

	var variantCount int64
	for _, p := range products {
		variants, err := s.Q.ListVariantsByProduct(ctx, sqlc.ListVariantsByProductParams{ProductID: p.ID, IsArchived: 0})
		if err != nil {
			serverError(w, err)
			return
		}
		variantCount += int64(len(variants))
	}

	u, _ := auth.UserFromContext(ctx)
	render(w, r, http.StatusOK, pages.Dashboard(
		int64(len(products)),
		variantCount,
		u.Role.CanViewFinancials(),
		CSRFToken(r),
	))
}
