package catalog

import (
	"net/http"
	"strconv"
	"strings"

	"database/sql"

	"github.com/a-h/templ"

	"github.com/druidswebdesign/varels-cms/internal/db/sqlc"
	"github.com/druidswebdesign/varels-cms/internal/handlers/common"
	"github.com/druidswebdesign/varels-cms/internal/handlers/middleware"
	"github.com/druidswebdesign/varels-cms/internal/views/flash"
	"github.com/druidswebdesign/varels-cms/internal/views/pages"
	"github.com/druidswebdesign/varels-cms/internal/views/partials"
)

// HandleVariantTable renders GET /products/{id}/variants as an HTMX fragment.
func (s *Server) HandleVariantTable(w http.ResponseWriter, r *http.Request) {
	id, err := common.URLID(r, "id")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	variants, err := s.Q.ListVariantsByProduct(r.Context(), sqlc.ListVariantsByProductParams{ProductID: id, IsArchived: 0})
	if err != nil {
		common.ServerError(w, err)
		return
	}
	common.Render(w, r, http.StatusOK, partials.VariantTable(id, variants, middleware.CSRFToken(r)))
}

// HandleVariantCreate handles POST /products/{id}/variants.
func (s *Server) HandleVariantCreate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	productID, err := common.URLID(r, "id")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	_ = r.ParseForm()

	sku := strings.TrimSpace(r.FormValue("sku"))
	if sku == "" {
		http.Redirect(w, r, "/products/"+common.Itoa(productID), http.StatusSeeOther)
		return
	}

	cost, err := common.ParsePesos(r.FormValue("cost"))
	if err != nil {
		http.Redirect(w, r, "/products/"+common.Itoa(productID)+"?err=cost", http.StatusSeeOther)
		return
	}
	retail, err := common.ParsePesos(r.FormValue("retail"))
	if err != nil {
		http.Redirect(w, r, "/products/"+common.Itoa(productID)+"?err=retail", http.StatusSeeOther)
		return
	}
	wholesaleRaw := strings.TrimSpace(r.FormValue("wholesale"))
	wholesale, err := common.ParsePesos(wholesaleRaw)
	if err != nil {
		http.Redirect(w, r, "/products/"+common.Itoa(productID)+"?err=wholesale", http.StatusSeeOther)
		return
	}

	var threshold int64
	if v := strings.TrimSpace(r.FormValue("threshold")); v != "" {
		threshold, _ = strconv.ParseInt(v, 10, 64)
	}

	if _, err := s.Q.CreateVariant(ctx, sqlc.CreateVariantParams{
		ProductID:           productID,
		Sku:                 sku,
		Size:                common.NullString(r.FormValue("size")),
		Color:               common.NullString(r.FormValue("color")),
		Barcode:             common.NullString(r.FormValue("barcode")),
		CostMinor:           cost,
		RetailPriceMinor:    retail,
		WholesalePriceMinor: sql.NullInt64{Int64: wholesale, Valid: wholesaleRaw != ""},
		LowStockThreshold:   threshold,
	}); err != nil {
		if common.IsUniqueViolation(err) {
			s.SetFlash(r, "That SKU is already in use.", "error")
			if common.IsHX(r) {
				s.renderVariantTable(w, r, productID, "That SKU is already in use.", "error")
				return
			}
			http.Redirect(w, r, "/products/"+common.Itoa(productID), http.StatusSeeOther)
			return
		}
		common.ServerError(w, err)
		return
	}

	s.SetFlash(r, "Variant added.", "success")
	if common.IsHX(r) {
		s.renderVariantTable(w, r, productID, "Variant added.", "success")
		return
	}
	http.Redirect(w, r, "/products/"+common.Itoa(productID), http.StatusSeeOther)
}

// renderVariantTable re-renders the variant fragment plus an out-of-band flash
// for HTMX mutation responses.
func (s *Server) renderVariantTable(w http.ResponseWriter, r *http.Request, productID int64, message, tone string) {
	variants, err := s.Q.ListVariantsByProduct(r.Context(), sqlc.ListVariantsByProductParams{ProductID: productID, IsArchived: 0})
	if err != nil {
		common.ServerError(w, err)
		return
	}
	common.Render(w, r, http.StatusOK, templ.Join(
		partials.VariantTable(productID, variants, middleware.CSRFToken(r)),
		partials.FlashOOB(flash.Flash{Message: message, Tone: tone}),
	))
}

// HandleVariantForm renders GET /variants/{id}/edit.
func (s *Server) HandleVariantForm(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, err := common.URLID(r, "id")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	v, err := s.Q.GetVariant(ctx, id)
	if err != nil {
		if common.IsNotFound(err) {
			http.NotFound(w, r)
			return
		}
		common.ServerError(w, err)
		return
	}
	common.Render(w, r, http.StatusOK, pages.VariantForm(v, middleware.CSRFToken(r)))
}

// HandleVariantUpdate handles POST /variants/{id}.
func (s *Server) HandleVariantUpdate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, err := common.URLID(r, "id")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	v, err := s.Q.GetVariant(ctx, id)
	if err != nil {
		if common.IsNotFound(err) {
			http.NotFound(w, r)
			return
		}
		common.ServerError(w, err)
		return
	}
	_ = r.ParseForm()

	sku := strings.TrimSpace(r.FormValue("sku"))
	if sku == "" {
		http.Redirect(w, r, "/variants/"+common.Itoa(id)+"/edit?err=sku", http.StatusSeeOther)
		return
	}
	cost, costErr := common.ParsePesos(r.FormValue("cost"))
	retail, retailErr := common.ParsePesos(r.FormValue("retail"))
	wholesaleRaw := strings.TrimSpace(r.FormValue("wholesale"))
	wholesale, wholesaleErr := common.ParsePesos(wholesaleRaw)
	if costErr != nil || retailErr != nil || wholesaleErr != nil {
		http.Redirect(w, r, "/variants/"+common.Itoa(id)+"/edit?err=price", http.StatusSeeOther)
		return
	}

	if err := s.Q.UpdateVariant(ctx, sqlc.UpdateVariantParams{
		Sku:                 sku,
		Size:                common.NullString(r.FormValue("size")),
		Color:               common.NullString(r.FormValue("color")),
		Barcode:             common.NullString(r.FormValue("barcode")),
		CostMinor:           cost,
		RetailPriceMinor:    retail,
		WholesalePriceMinor: sql.NullInt64{Int64: wholesale, Valid: wholesaleRaw != ""},
		ID:                  id,
	}); err != nil {
		if common.IsUniqueViolation(err) {
			http.Redirect(w, r, "/variants/"+common.Itoa(id)+"/edit?err=duplicate", http.StatusSeeOther)
			return
		}
		common.ServerError(w, err)
		return
	}

	http.Redirect(w, r, "/products/"+common.Itoa(v.ProductID), http.StatusSeeOther)
}

// HandleVariantArchive handles POST /variants/{id}/archive.
func (s *Server) HandleVariantArchive(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, err := common.URLID(r, "id")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	v, err := s.Q.GetVariant(ctx, id)
	if err != nil {
		if common.IsNotFound(err) {
			http.NotFound(w, r)
			return
		}
		common.ServerError(w, err)
		return
	}
	if err := s.Q.SetVariantArchived(ctx, sqlc.SetVariantArchivedParams{IsArchived: 1, ID: id}); err != nil {
		common.ServerError(w, err)
		return
	}
	if common.IsHX(r) {
		s.renderVariantTable(w, r, v.ProductID, "Variant archived.", "warning")
		return
	}
	s.SetFlash(r, "Variant archived.", "warning")
	http.Redirect(w, r, "/products/"+common.Itoa(v.ProductID), http.StatusSeeOther)
}

// HandleVariantThreshold handles POST /variants/{id}/threshold.
func (s *Server) HandleVariantThreshold(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, err := common.URLID(r, "id")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	v, err := s.Q.GetVariant(ctx, id)
	if err != nil {
		if common.IsNotFound(err) {
			http.NotFound(w, r)
			return
		}
		common.ServerError(w, err)
		return
	}
	threshold, _ := strconv.ParseInt(strings.TrimSpace(r.FormValue("threshold")), 10, 64)
	if err := s.Q.SetVariantThreshold(ctx, sqlc.SetVariantThresholdParams{LowStockThreshold: threshold, ID: id}); err != nil {
		common.ServerError(w, err)
		return
	}
	if common.IsHX(r) {
		s.renderVariantTable(w, r, v.ProductID, "Threshold updated.", "success")
		return
	}
	s.SetFlash(r, "Threshold updated.", "success")
	http.Redirect(w, r, "/products/"+common.Itoa(v.ProductID), http.StatusSeeOther)
}
