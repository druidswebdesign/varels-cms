package handlers

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/yourname/varels_cms/internal/auth"
	"github.com/yourname/varels_cms/internal/db/sqlc"
)

// adminOnlyExports require the admin role (functions.md §3, routes.md §3.9).
var adminOnlyExports = map[string]bool{
	"margins":  true,
	"pl":       true,
	"expenses": true,
}

// HandleExport streams a CSV for /export/{resource}.csv.
func (s *Server) HandleExport(w http.ResponseWriter, r *http.Request) {
	resource := strings.TrimSuffix(strings.TrimSpace(chi.URLParam(r, "resource")), ".csv")
	if resource == "" {
		http.NotFound(w, r)
		return
	}
	if adminOnlyExports[resource] {
		u, _ := auth.UserFromContext(r.Context())
		if !u.Role.CanViewFinancials() {
			http.Error(w, "forbidden: admin role required", http.StatusForbidden)
			return
		}
	}

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition",
		fmt.Sprintf("attachment; filename=\"%s-%s.csv\"", resource, time.Now().UTC().Format("20060102")))

	cw := csv.NewWriter(w)
	defer cw.Flush()

	if err := s.writeExport(r, cw, resource); err != nil {
		// Headers are already sent; log via a final row rather than a 500.
		_ = cw.Write([]string{"error", err.Error()})
	}
}

func (s *Server) writeExport(r *http.Request, cw *csv.Writer, resource string) error {
	ctx := r.Context()

	switch resource {
	case "products":
		if err := cw.Write([]string{"id", "name", "slug", "category_id", "is_archived", "created_at"}); err != nil {
			return err
		}
		for _, archived := range []int64{0, 1} {
			rows, err := s.Q.ListProducts(ctx, sqlc.ListProductsParams{IsArchived: archived, CategoryID: nil, Offset: 0, Limit: 100000})
			if err != nil {
				return err
			}
			for _, p := range rows {
				if err := cw.Write([]string{
					itoa(p.ID), p.Name, p.Slug, nullableInt(p.CategoryID.Int64, p.CategoryID.Valid),
					strconv.FormatInt(p.IsArchived, 10), p.CreatedAt,
				}); err != nil {
					return err
				}
			}
		}

	case "variants", "margins":
		header := []string{"id", "product", "sku", "size", "color", "cost_minor", "retail_minor", "wholesale_minor", "low_stock_threshold"}
		if resource == "margins" {
			header = append(header, "gross_margin_bps")
		}
		if err := cw.Write(header); err != nil {
			return err
		}
		rows, err := s.Q.ListSellableVariants(ctx)
		if err != nil {
			return err
		}
		for _, v := range rows {
			rec := []string{
				itoa(v.ID), v.ProductName, v.Sku, v.Size.String, v.Color.String,
				strconv.FormatInt(v.CostMinor, 10), strconv.FormatInt(v.RetailPriceMinor, 10),
				nullableInt(v.WholesalePriceMinor.Int64, v.WholesalePriceMinor.Valid),
				strconv.FormatInt(v.LowStockThreshold, 10),
			}
			if resource == "margins" {
				rec = append(rec, strconv.FormatInt(marginBps(v.RetailPriceMinor, v.CostMinor), 10))
			}
			if err := cw.Write(rec); err != nil {
				return err
			}
		}

	case "sales":
		if err := cw.Write([]string{"id", "order_number", "placed_at", "customer_id", "channel_id", "location_id", "status", "subtotal_minor", "discount_minor", "tax_minor", "shipping_minor", "total_minor", "payment_status"}); err != nil {
			return err
		}
		orders, err := s.Q.ListOrders(ctx, sqlc.ListOrdersParams{Limit: 100000, Offset: 0})
		if err != nil {
			return err
		}
		for _, o := range orders {
			if err := cw.Write([]string{
				itoa(o.ID), o.OrderNumber, o.PlacedAt,
				nullableInt(o.CustomerID.Int64, o.CustomerID.Valid),
				strconv.FormatInt(o.ChannelID, 10), strconv.FormatInt(o.LocationID, 10),
				string(o.Status), strconv.FormatInt(o.SubtotalMinor, 10), strconv.FormatInt(o.DiscountMinor, 10),
				strconv.FormatInt(o.TaxMinor, 10), strconv.FormatInt(o.ShippingMinor, 10),
				strconv.FormatInt(o.TotalMinor, 10), string(o.PaymentStatus),
			}); err != nil {
				return err
			}
		}

	case "stock-movements":
		if err := cw.Write([]string{"id", "created_at", "variant_id", "location_id", "state", "quantity_delta", "ref_type", "ref_id", "reason_code", "note"}); err != nil {
			return err
		}
		rows, err := s.Q.ListStockMovements(ctx, sqlc.ListStockMovementsParams{Limit: 100000, Offset: 0})
		if err != nil {
			return err
		}
		for _, m := range rows {
			if err := cw.Write([]string{
				itoa(m.ID), m.CreatedAt, strconv.FormatInt(m.VariantID, 10), strconv.FormatInt(m.LocationID, 10),
				string(m.State), strconv.FormatInt(m.QuantityDelta, 10), string(m.RefType),
				nullableInt(m.RefID.Int64, m.RefID.Valid), m.ReasonCode.String, m.Note.String,
			}); err != nil {
				return err
			}
		}

	case "expenses":
		if err := cw.Write([]string{"id", "category_id", "vendor", "amount_minor", "incurred_at", "note"}); err != nil {
			return err
		}
		rows, err := s.Q.ListExpenses(ctx, sqlc.ListExpensesParams{Limit: 100000, Offset: 0})
		if err != nil {
			return err
		}
		for _, e := range rows {
			if err := cw.Write([]string{
				itoa(e.ID), strconv.FormatInt(e.CategoryID, 10), e.Vendor.String,
				strconv.FormatInt(e.AmountMinor, 10), e.IncurredAt, e.Note.String,
			}); err != nil {
				return err
			}
		}

	case "pl":
		now := time.Now().In(art)
		from, to := periodRange(now.Year(), int(now.Month()))
		summary, err := s.Q.SalesSummary(ctx, sqlc.SalesSummaryParams{FromTs: from, ToTs: to})
		if err != nil {
			return err
		}
		cogs := asInt64Or(s.Q.COGSSummary(ctx, sqlc.COGSSummaryParams{FromTs: from, ToTs: to}))
		opex := asInt64Or(s.Q.SumExpenses(ctx, sqlc.SumExpensesParams{FromTs: from, ToTs: to}))
		revenue := asInt64(summary.RevenueMinor)
		gross := revenue - cogs
		if err := cw.Write([]string{"metric", "amount_minor"}); err != nil {
			return err
		}
		for _, row := range [][2]interface{}{
			{"revenue", revenue},
			{"cogs", cogs},
			{"gross_profit", gross},
			{"opex", opex},
			{"net_profit", gross - opex},
		} {
			if err := cw.Write([]string{row[0].(string), strconv.FormatInt(row[1].(int64), 10)}); err != nil {
				return err
			}
		}

	default:
		return fmt.Errorf("unknown resource %q", resource)
	}
	return nil
}

func nullableInt(v int64, valid bool) string {
	if !valid {
		return ""
	}
	return strconv.FormatInt(v, 10)
}

// marginBps returns gross margin in basis points (half-up).
func marginBps(retail, cost int64) int64 {
	if retail <= 0 {
		return 0
	}
	return ((retail-cost)*10000 + retail/2) / retail
}
