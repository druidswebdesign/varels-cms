package handlers

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/a-h/templ"

	"github.com/yourname/varels_cms/internal/db/sqlc"
	"github.com/yourname/varels_cms/internal/views/pages"
	"github.com/yourname/varels_cms/internal/views/partials"
	"github.com/yourname/varels_cms/internal/views/vm"
)

const bestSellersLimit = 15

// HandleProfitCards renders the admin P&L cards for the current month.
func (s *Server) HandleProfitCards(w http.ResponseWriter, r *http.Request) {
	now := time.Now().In(art)
	from, to := periodRange(now.Year(), int(now.Month()))
	render(w, r, http.StatusOK, s.profitCards(r.Context(), from, to))
}

// HandleAnalytics renders the analytics page for a year/month.
func (s *Server) HandleAnalytics(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	year, month := periodFromQuery(r)
	from, to := periodRange(year, month)

	summary, err := s.Q.SalesSummary(ctx, sqlc.SalesSummaryParams{FromTs: from, ToTs: to})
	if err != nil {
		serverError(w, err)
		return
	}
	cogs := asInt64Or(s.Q.COGSSummary(ctx, sqlc.COGSSummaryParams{FromTs: from, ToTs: to}))
	opex := asInt64Or(s.Q.SumExpenses(ctx, sqlc.SumExpensesParams{FromTs: from, ToTs: to}))
	revenue := asInt64(summary.RevenueMinor)

	byChannel, err := s.Q.RevenueByChannel(ctx, sqlc.RevenueByChannelParams{FromTs: from, ToTs: to})
	if err != nil {
		serverError(w, err)
		return
	}
	best, err := s.Q.BestSellers(ctx, sqlc.BestSellersParams{FromTs: from, ToTs: to, Limit: bestSellersLimit})
	if err != nil {
		serverError(w, err)
		return
	}
	sizes, err := s.Q.BestSellingSizes(ctx, sqlc.BestSellingSizesParams{FromTs: from, ToTs: to})
	if err != nil {
		serverError(w, err)
		return
	}
	valuation, err := s.Q.InventoryValuation(ctx)
	if err != nil {
		serverError(w, err)
		return
	}

	pl := vm.PL{
		RevenueMinor: revenue,
		COGSMinor:    cogs,
		GrossMinor:   revenue - cogs,
		OpExMinor:    opex,
		NetMinor:     revenue - cogs - opex,
		OrderCount:   summary.OrderCount,
	}

	render(w, r, http.StatusOK, pages.Analytics(
		year, month, pl,
		toChannelRevenue(byChannel),
		toBestSellers(best),
		toSizeRows(sizes),
		vm.Valuation{
			InventoryCostMinor:   asInt64(valuation.InventoryCostMinor),
			PotentialRetailMinor: asInt64(valuation.PotentialRetailMinor),
			NonSellableCostMinor: asInt64(valuation.NonSellableCostMinor),
		},
		CSRFToken(r),
	))
}

// HandleBestSellers renders the best-sellers fragment.
func (s *Server) HandleBestSellers(w http.ResponseWriter, r *http.Request) {
	from, to, period := rangeFromPeriod(r)
	rows, err := s.Q.BestSellers(r.Context(), sqlc.BestSellersParams{FromTs: from, ToTs: to, Limit: bestSellersLimit})
	if err != nil {
		serverError(w, err)
		return
	}
	render(w, r, http.StatusOK, partials.BestSellersTable(toBestSellers(rows), period))
}

// HandleSalesChart renders the revenue-by-channel fragment.
func (s *Server) HandleSalesChart(w http.ResponseWriter, r *http.Request) {
	from, to, _ := rangeFromPeriod(r)
	rows, err := s.Q.RevenueByChannel(r.Context(), sqlc.RevenueByChannelParams{FromTs: from, ToTs: to})
	if err != nil {
		serverError(w, err)
		return
	}
	render(w, r, http.StatusOK, partials.SalesChart(toChannelRevenue(rows)))
}

// HandleMargin renders the P&L cards fragment for a period.
func (s *Server) HandleMargin(w http.ResponseWriter, r *http.Request) {
	from, to, _ := rangeFromPeriod(r)
	render(w, r, http.StatusOK, s.profitCards(r.Context(), from, to))
}

// HandleSizeCurve renders the best-selling sizes fragment.
func (s *Server) HandleSizeCurve(w http.ResponseWriter, r *http.Request) {
	from, to, _ := rangeFromPeriod(r)
	rows, err := s.Q.BestSellingSizes(r.Context(), sqlc.BestSellingSizesParams{FromTs: from, ToTs: to})
	if err != nil {
		serverError(w, err)
		return
	}
	render(w, r, http.StatusOK, partials.SizeCurve(toSizeRows(rows)))
}

// HandleProfitLoss redirects to the analytics page (P&L is part of it).
func (s *Server) HandleProfitLoss(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/analytics?"+r.URL.RawQuery, http.StatusSeeOther)
}

// profitCards computes a P&L period and renders the cards partial.
func (s *Server) profitCards(ctx context.Context, from, to string) templ.Component {
	summary, err := s.Q.SalesSummary(ctx, sqlc.SalesSummaryParams{FromTs: from, ToTs: to})
	if err != nil {
		log.Printf("profit cards: %v", err)
		return partials.ProfitCards(0, 0, 0, 0, 0)
	}
	revenue := asInt64(summary.RevenueMinor)
	cogs := asInt64Or(s.Q.COGSSummary(ctx, sqlc.COGSSummaryParams{FromTs: from, ToTs: to}))
	opex := asInt64Or(s.Q.SumExpenses(ctx, sqlc.SumExpensesParams{FromTs: from, ToTs: to}))
	gross := revenue - cogs
	return partials.ProfitCards(revenue, cogs, gross, opex, gross-opex)
}

func toBestSellers(rows []sqlc.BestSellersRow) []vm.BestSeller {
	out := make([]vm.BestSeller, 0, len(rows))
	for _, r := range rows {
		out = append(out, vm.BestSeller{
			VariantID:    r.VariantID,
			Sku:          r.Sku,
			ProductName:  r.ProductName,
			Size:         r.Size.String,
			Color:        r.Color.String,
			UnitsSold:    nullFloatToInt64(r.UnitsSold),
			RevenueMinor: nullFloatToInt64(r.RevenueMinor),
		})
	}
	return out
}

func toChannelRevenue(rows []sqlc.RevenueByChannelRow) []vm.ChannelRevenue {
	out := make([]vm.ChannelRevenue, 0, len(rows))
	for _, r := range rows {
		out = append(out, vm.ChannelRevenue{
			Channel:      r.Channel,
			OrderCount:   r.OrderCount,
			RevenueMinor: asInt64(r.RevenueMinor),
		})
	}
	return out
}

func toSizeRows(rows []sqlc.BestSellingSizesRow) []vm.SizeRow {
	out := make([]vm.SizeRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, vm.SizeRow{Size: r.Size.String, UnitsSold: nullFloatToInt64(r.UnitsSold)})
	}
	return out
}

func nullFloatToInt64(n sql.NullFloat64) int64 {
	if !n.Valid {
		return 0
	}
	return int64(n.Float64)
}

// rangeFromPeriod resolves ?period=year|month|all, falling back to
// ?year=&month= (month 0 = whole year) when no period is given, so the
// analytics filter inputs can drive the chart/sizes fragments.
func rangeFromPeriod(r *http.Request) (string, string, string) {
	now := time.Now().In(art)
	switch strings.TrimSpace(r.URL.Query().Get("period")) {
	case "year":
		from, to := periodRange(now.Year(), 0)
		return from, to, "year"
	case "all":
		from, to := allTimeRange()
		return from, to, "all"
	case "month":
		from, to := periodRange(now.Year(), int(now.Month()))
		return from, to, "month"
	}
	year, month := periodFromQuery(r)
	from, to := periodRange(year, month)
	label := "month"
	if month == 0 {
		label = "year"
	}
	return from, to, label
}

// periodFromQuery resolves ?year=&month= (month 0 = whole year, default current).
func periodFromQuery(r *http.Request) (int, int) {
	now := time.Now().In(art)
	year, month := now.Year(), int(now.Month())
	if v, ok := parseInt64(r.URL.Query().Get("year")); ok && v > 0 {
		year = int(v)
	}
	if v, ok := parseInt64(r.URL.Query().Get("month")); ok && v >= 0 && v <= 12 {
		month = int(v)
	}
	return year, month
}
