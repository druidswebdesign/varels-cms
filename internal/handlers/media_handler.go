package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/yourname/varels_cms/internal/db/sqlc"
	"github.com/yourname/varels_cms/internal/db/types"
)

// HandleMediaCreate handles POST /products/{id}/media.
func (s *Server) HandleMediaCreate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	productID, err := urlID(r, "id")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if _, err := s.Q.GetProduct(ctx, productID); err != nil {
		if isNotFound(err) {
			http.NotFound(w, r)
			return
		}
		serverError(w, err)
		return
	}
	_ = r.ParseForm()

	mediaURL := strings.TrimSpace(r.FormValue("url"))
	if mediaURL == "" {
		s.setFlash(r, "Media URL is required.", "error")
		http.Redirect(w, r, "/products/"+itoa(productID), http.StatusSeeOther)
		return
	}
	mediaType := types.MediaType(strings.TrimSpace(r.FormValue("media_type")))
	switch mediaType {
	case types.MediaTypeImage, types.MediaTypeSizeChart, types.MediaTypeVideo:
	default:
		mediaType = types.MediaTypeImage
	}

	var sortOrder int64
	if v := strings.TrimSpace(r.FormValue("sort_order")); v != "" {
		sortOrder, _ = strconv.ParseInt(v, 10, 64)
	}
	isPrimary := int64(0)
	if r.FormValue("is_primary") == "1" {
		isPrimary = 1
		if err := s.Q.ClearPrimaryMedia(ctx, productID); err != nil {
			serverError(w, err)
			return
		}
	}

	if _, err := s.Q.CreateProductMedia(ctx, sqlc.CreateProductMediaParams{
		ProductID: productID,
		VariantID: nullInt64(r.FormValue("variant_id")),
		MediaType: mediaType,
		Url:       mediaURL,
		AltText:   nullString(r.FormValue("alt_text")),
		SortOrder: sortOrder,
		IsPrimary: isPrimary,
	}); err != nil {
		serverError(w, err)
		return
	}

	s.audit(r, "create", "product_media", productID, "media")
	s.setFlash(r, "Media added.", "success")
	http.Redirect(w, r, "/products/"+itoa(productID), http.StatusSeeOther)
}

// HandleMediaDelete handles POST /media/{id}/delete.
func (s *Server) HandleMediaDelete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, err := urlID(r, "id")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	media, err := s.Q.GetProductMedia(ctx, id)
	if err != nil {
		if isNotFound(err) {
			http.NotFound(w, r)
			return
		}
		serverError(w, err)
		return
	}
	if err := s.Q.DeleteProductMedia(ctx, id); err != nil {
		serverError(w, err)
		return
	}
	s.audit(r, "delete", "product_media", media.ProductID, "media")
	s.setFlash(r, "Media deleted.", "warning")
	http.Redirect(w, r, "/products/"+itoa(media.ProductID), http.StatusSeeOther)
}
