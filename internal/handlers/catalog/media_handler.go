package catalog

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/yourname/varels_cms/internal/db/sqlc"
	"github.com/yourname/varels_cms/internal/db/types"
	"github.com/yourname/varels_cms/internal/handlers/common"
)

// HandleMediaCreate handles POST /products/{id}/media.
func (s *Server) HandleMediaCreate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	productID, err := common.URLID(r, "id")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if _, err := s.Q.GetProduct(ctx, productID); err != nil {
		if common.IsNotFound(err) {
			http.NotFound(w, r)
			return
		}
		common.ServerError(w, err)
		return
	}
	_ = r.ParseForm()

	mediaURL := strings.TrimSpace(r.FormValue("url"))
	if mediaURL == "" {
		s.SetFlash(r, "Media URL is required.", "error")
		http.Redirect(w, r, "/products/"+common.Itoa(productID), http.StatusSeeOther)
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
			common.ServerError(w, err)
			return
		}
	}

	if _, err := s.Q.CreateProductMedia(ctx, sqlc.CreateProductMediaParams{
		ProductID: productID,
		VariantID: common.NullInt64(r.FormValue("variant_id")),
		MediaType: mediaType,
		Url:       mediaURL,
		AltText:   common.NullString(r.FormValue("alt_text")),
		SortOrder: sortOrder,
		IsPrimary: isPrimary,
	}); err != nil {
		common.ServerError(w, err)
		return
	}

	s.Audit(r, "create", "product_media", productID, "media")
	s.SetFlash(r, "Media added.", "success")
	http.Redirect(w, r, "/products/"+common.Itoa(productID), http.StatusSeeOther)
}

// HandleMediaDelete handles POST /media/{id}/delete.
func (s *Server) HandleMediaDelete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, err := common.URLID(r, "id")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	media, err := s.Q.GetProductMedia(ctx, id)
	if err != nil {
		if common.IsNotFound(err) {
			http.NotFound(w, r)
			return
		}
		common.ServerError(w, err)
		return
	}
	if err := s.Q.DeleteProductMedia(ctx, id); err != nil {
		common.ServerError(w, err)
		return
	}
	s.Audit(r, "delete", "product_media", media.ProductID, "media")
	s.SetFlash(r, "Media deleted.", "warning")
	http.Redirect(w, r, "/products/"+common.Itoa(media.ProductID), http.StatusSeeOther)
}
