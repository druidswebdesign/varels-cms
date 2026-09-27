package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"database/sql"

	"github.com/a-h/templ"

	"github.com/yourname/varels_cms/internal/db/sqlc"
	"github.com/yourname/varels_cms/internal/views/flash"
	"github.com/yourname/varels_cms/internal/views/pages"
	"github.com/yourname/varels_cms/internal/views/partials"
)

// HandleVariantTable renders GET /products/{id}/variants as an HTMX fragment.
func (s *Server) HandleVariantTable(w http.ResponseWriter, r *http.Request) {
	id, err := urlID(r, "id")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	variants, err := s.Q.ListVariantsByProduct(r.Context(), sqlc.ListVariantsByProductParams{ProductID: id, IsArchived: 0})
	if err != nil {
		serverError(w, err)
		return
	}
	render(w, r, http.StatusOK, partials.VariantTable(id, variants, CSRFToken(r)))
}

// HandleVariantCreate handles POST /products/{id}/variants.
func (s *Server) HandleVariantCreate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	productID, err := urlID(r, "id")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	_ = r.ParseForm()

	sku := strings.TrimSpace(r.FormValue("sku"))
	if sku == "" {
		http.Redirect(w, r, "/products/"+itoa(productID), http.StatusSeeOther)
		return
	}

	cost, err := parsePesos(r.FormValue("cost"))
	if err != nil {
		http.Redirect(w, r, "/products/"+itoa(productID)+"?err=cost", http.StatusSeeOther)
		return
	}
	retail, err := parsePesos(r.FormValue("retail"))
	if err != nil {
		http.Redirect(w, r, "/products/"+itoa(productID)+"?err=retail", http.StatusSeeOther)
		return
	}
	wholesaleRaw := strings.TrimSpace(r.FormValue("wholesale"))
	wholesale, err := parsePesos(wholesaleRaw)
	if err != nil {
		http.Redirect(w, r, "/products/"+itoa(productID)+"?err=wholesale", http.StatusSeeOther)
		return
	}

	var threshold int64
	if v := strings.TrimSpace(r.FormValue("threshold")); v != "" {
		threshold, _ = strconv.ParseInt(v, 10, 64)
	}

	if _, err := s.Q.CreateVariant(ctx, sqlc.CreateVariantParams{
		ProductID:           productID,
		Sku:                 sku,
		Size:                nullString(r.FormValue("size")),
		Color:               nullString(r.FormValue("color")),
		Barcode:             nullString(r.FormValue("barcode")),
		CostMinor:           cost,
		RetailPriceMinor:    retail,
		WholesalePriceMinor: sql.NullInt64{Int64: wholesale, Valid: wholesaleRaw != ""},
		LowStockThreshold:   threshold,
	}); err != nil {
		if isUniqueViolation(err) {
			s.setFlash(r, "That SKU is already in use.", "error")
			if isHX(r) {
				s.renderVariantTable(w, r, productID, "That SKU is already in use.", "error")
				return
			}
			http.Redirect(w, r, "/products/"+itoa(productID), http.StatusSeeOther)
			return
		}
		serverError(w, err)
		return
	}

	s.setFlash(r, "Variant added.", "success")
	if isHX(r) {
		s.renderVariantTable(w, r, productID, "Variant added.", "success")
		return
	}
	http.Redirect(w, r, "/products/"+itoa(productID), http.StatusSeeOther)
}

// renderVariantTable re-renders the variant fragment plus an out-of-band flash
// for HTMX mutation responses.
func (s *Server) renderVariantTable(w http.ResponseWriter, r *http.Request, productID int64, message, tone string) {
	variants, err := s.Q.ListVariantsByProduct(r.Context(), sqlc.ListVariantsByProductParams{ProductID: productID, IsArchived: 0})
	if err != nil {
		serverError(w, err)
		return
	}
	render(w, r, http.StatusOK, templ.Join(
		partials.VariantTable(productID, variants, CSRFToken(r)),
		partials.FlashOOB(flash.Flash{Message: message, Tone: tone}),
	))
}

// HandleVariantForm renders GET /variants/{id}/edit.
func (s *Server) HandleVariantForm(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, err := urlID(r, "id")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	v, err := s.Q.GetVariant(ctx, id)
	if err != nil {
		if isNotFound(err) {
			http.NotFound(w, r)
			return
		}
		serverError(w, err)
		return
	}
	render(w, r, http.StatusOK, pages.VariantForm(v, CSRFToken(r)))
}

// HandleVariantUpdate handles POST /variants/{id}.
func (s *Server) HandleVariantUpdate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, err := urlID(r, "id")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	v, err := s.Q.GetVariant(ctx, id)
	if err != nil {
		if isNotFound(err) {
			http.NotFound(w, r)
			return
		}
		serverError(w, err)
		return
	}
	_ = r.ParseForm()

	sku := strings.TrimSpace(r.FormValue("sku"))
	if sku == "" {
		http.Redirect(w, r, "/variants/"+itoa(id)+"/edit?err=sku", http.StatusSeeOther)
		return
	}
	cost, costErr := parsePesos(r.FormValue("cost"))
	retail, retailErr := parsePesos(r.FormValue("retail"))
	wholesaleRaw := strings.TrimSpace(r.FormValue("wholesale"))
	wholesale, wholesaleErr := parsePesos(wholesaleRaw)
	if costErr != nil || retailErr != nil || wholesaleErr != nil {
		http.Redirect(w, r, "/variants/"+itoa(id)+"/edit?err=price", http.StatusSeeOther)
		return
	}

	if err := s.Q.UpdateVariant(ctx, sqlc.UpdateVariantParams{
		Sku:                 sku,
		Size:                nullString(r.FormValue("size")),
		Color:               nullString(r.FormValue("color")),
		Barcode:             nullString(r.FormValue("barcode")),
		CostMinor:           cost,
		RetailPriceMinor:    retail,
		WholesalePriceMinor: sql.NullInt64{Int64: wholesale, Valid: wholesaleRaw != ""},
		ID:                  id,
	}); err != nil {
		if isUniqueViolation(err) {
			http.Redirect(w, r, "/variants/"+itoa(id)+"/edit?err=duplicate", http.StatusSeeOther)
			return
		}
		serverError(w, err)
		return
	}

	http.Redirect(w, r, "/products/"+itoa(v.ProductID), http.StatusSeeOther)
}

// HandleVariantArchive handles POST /variants/{id}/archive.
func (s *Server) HandleVariantArchive(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, err := urlID(r, "id")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	v, err := s.Q.GetVariant(ctx, id)
	if err != nil {
		if isNotFound(err) {
			http.NotFound(w, r)
			return
		}
		serverError(w, err)
		return
	}
	if err := s.Q.SetVariantArchived(ctx, sqlc.SetVariantArchivedParams{IsArchived: 1, ID: id}); err != nil {
		serverError(w, err)
		return
	}
	if isHX(r) {
		s.renderVariantTable(w, r, v.ProductID, "Variant archived.", "warning")
		return
	}
	s.setFlash(r, "Variant archived.", "warning")
	http.Redirect(w, r, "/products/"+itoa(v.ProductID), http.StatusSeeOther)
}

// HandleVariantThreshold handles POST /variants/{id}/threshold.
func (s *Server) HandleVariantThreshold(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, err := urlID(r, "id")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	v, err := s.Q.GetVariant(ctx, id)
	if err != nil {
		if isNotFound(err) {
			http.NotFound(w, r)
			return
		}
		serverError(w, err)
		return
	}
	threshold, _ := strconv.ParseInt(strings.TrimSpace(r.FormValue("threshold")), 10, 64)
	if err := s.Q.SetVariantThreshold(ctx, sqlc.SetVariantThresholdParams{LowStockThreshold: threshold, ID: id}); err != nil {
		serverError(w, err)
		return
	}
	if isHX(r) {
		s.renderVariantTable(w, r, v.ProductID, "Threshold updated.", "success")
		return
	}
	s.setFlash(r, "Threshold updated.", "success")
	http.Redirect(w, r, "/products/"+itoa(v.ProductID), http.StatusSeeOther)
}
