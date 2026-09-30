package admin

import (
	"net/http"

	"github.com/druidswebdesign/varels-cms/internal/auth"
	"github.com/druidswebdesign/varels-cms/internal/db/sqlc"
	"github.com/druidswebdesign/varels-cms/internal/handlers/common"
	"github.com/druidswebdesign/varels-cms/internal/handlers/middleware"
	"github.com/druidswebdesign/varels-cms/internal/views/pages"
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
		Limit:      common.ProductListLimit,
	})
	if err != nil {
		common.ServerError(w, err)
		return
	}

	var variantCount int64
	for _, p := range products {
		variants, err := s.Q.ListVariantsByProduct(ctx, sqlc.ListVariantsByProductParams{ProductID: p.ID, IsArchived: 0})
		if err != nil {
			common.ServerError(w, err)
			return
		}
		variantCount += int64(len(variants))
	}

	u, _ := auth.UserFromContext(ctx)
	common.Render(w, r, http.StatusOK, pages.Dashboard(
		int64(len(products)),
		variantCount,
		u.Role.CanViewFinancials(),
		middleware.CSRFToken(r),
	))
}
