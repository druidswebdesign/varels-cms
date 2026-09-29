package catalog

import (
	"database/sql"
	"net/http"
	"strings"

	"github.com/yourname/varels_cms/internal/db/sqlc"
	"github.com/yourname/varels_cms/internal/handlers/common"
	"github.com/yourname/varels_cms/internal/handlers/middleware"
	"github.com/yourname/varels_cms/internal/service"
	"github.com/yourname/varels_cms/internal/views/pages"
	"github.com/yourname/varels_cms/internal/views/partials"
)

// HandleProductsList renders GET /products with optional search and archived
// filter.
func (s *Server) HandleProductsList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	archived := r.URL.Query().Get("archived") == "1"

	categories, err := s.Q.ListCategories(ctx)
	if err != nil {
		common.ServerError(w, err)
		return
	}

	var products []sqlc.Product
	if q != "" {
		products, err = s.Q.SearchProducts(ctx, sqlc.SearchProductsParams{
			Q:     "%" + q + "%",
			Limit: common.ProductListLimit,
		})
	} else {
		isArchived := int64(0)
		if archived {
			isArchived = 1
		}
		products, err = s.Q.ListProducts(ctx, sqlc.ListProductsParams{
			IsArchived: isArchived,
			CategoryID: nil,
			Offset:     0,
			Limit:      common.ProductListLimit,
		})
	}
	if err != nil {
		common.ServerError(w, err)
		return
	}

	common.Render(w, r, http.StatusOK, pages.ProductsList(products, categories, q, archived, middleware.CSRFToken(r)))
}

// HandleProductsSearch renders GET /products/search as an HTMX fragment: the
// rows of the products table for live search.
func (s *Server) HandleProductsSearch(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := strings.TrimSpace(r.URL.Query().Get("q"))

	var products []sqlc.Product
	var err error
	if q != "" {
		products, err = s.Q.SearchProducts(ctx, sqlc.SearchProductsParams{
			Q:     "%" + q + "%",
			Limit: common.ProductListLimit,
		})
	} else {
		products, err = s.Q.ListProducts(ctx, sqlc.ListProductsParams{
			IsArchived: 0,
			CategoryID: nil,
			Offset:     0,
			Limit:      common.ProductListLimit,
		})
	}
	if err != nil {
		common.ServerError(w, err)
		return
	}
	common.Render(w, r, http.StatusOK, partials.ProductRows(products, middleware.CSRFToken(r)))
}

// HandleProductForm renders the create form at GET /products/new.
func (s *Server) HandleProductForm(w http.ResponseWriter, r *http.Request) {
	categories, err := s.Q.ListCategories(r.Context())
	if err != nil {
		common.ServerError(w, err)
		return
	}
	common.Render(w, r, http.StatusOK, pages.ProductForm(sqlc.Product{}, categories, false, nil, middleware.CSRFToken(r)))
}

// HandleProductCreate handles POST /products.
func (s *Server) HandleProductCreate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	_ = r.ParseForm()

	name := strings.TrimSpace(r.FormValue("name"))
	slug := strings.TrimSpace(r.FormValue("slug"))
	if slug == "" {
		slug = common.Slugify(name)
	}
	description := common.NullString(r.FormValue("description"))
	categoryID := common.NullInt64(r.FormValue("category_id"))

	if errs := validateProduct(name, slug); len(errs) > 0 {
		s.reRenderProductForm(w, r, sqlc.Product{}, false, errs)
		return
	}

	p, err := s.Q.CreateProduct(ctx, sqlc.CreateProductParams{
		Name:        name,
		Slug:        slug,
		Description: description,
		CategoryID:  categoryID,
	})
	if err != nil {
		if common.IsUniqueViolation(err) {
			s.reRenderProductForm(w, r, sqlc.Product{Name: name, Slug: slug, Description: description}, false,
				map[string]string{"slug": "That slug is already taken."})
			return
		}
		common.ServerError(w, err)
		return
	}

	s.SetFlash(r, "Product created.", "success")
	http.Redirect(w, r, "/products/"+common.Itoa(p.ID), http.StatusSeeOther)
}

// HandleProductDetail renders GET /products/{id}.
func (s *Server) HandleProductDetail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, err := common.URLID(r, "id")
	if err != nil {
		http.NotFound(w, r)
		return
	}

	p, err := s.Q.GetProduct(ctx, id)
	if err != nil {
		if common.IsNotFound(err) {
			http.NotFound(w, r)
			return
		}
		common.ServerError(w, err)
		return
	}

	categoryName := "—"
	if p.CategoryID.Valid {
		if c, err := s.Q.GetCategory(ctx, p.CategoryID.Int64); err == nil {
			categoryName = c.Name
		}
	}

	variants, err := s.Q.ListVariantsByProduct(ctx, sqlc.ListVariantsByProductParams{ProductID: id, IsArchived: 0})
	if err != nil {
		common.ServerError(w, err)
		return
	}
	media, err := s.Q.ListProductMedia(ctx, id)
	if err != nil {
		common.ServerError(w, err)
		return
	}
	collections, err := s.Q.ListProductCollections(ctx, id)
	if err != nil {
		common.ServerError(w, err)
		return
	}
	allCollections, err := s.Q.ListCollections(ctx, 0)
	if err != nil {
		common.ServerError(w, err)
		return
	}

	common.Render(w, r, http.StatusOK, pages.ProductDetail(p, categoryName, variants, media, collections, allCollections, middleware.CSRFToken(r)))
}

// HandleProductEdit renders GET /products/{id}/edit.
func (s *Server) HandleProductEdit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, err := common.URLID(r, "id")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	p, err := s.Q.GetProduct(ctx, id)
	if err != nil {
		if common.IsNotFound(err) {
			http.NotFound(w, r)
			return
		}
		common.ServerError(w, err)
		return
	}
	categories, err := s.Q.ListCategories(ctx)
	if err != nil {
		common.ServerError(w, err)
		return
	}
	common.Render(w, r, http.StatusOK, pages.ProductForm(p, categories, true, nil, middleware.CSRFToken(r)))
}

// HandleProductUpdate handles POST /products/{id}.
func (s *Server) HandleProductUpdate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, err := common.URLID(r, "id")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	_ = r.ParseForm()

	name := strings.TrimSpace(r.FormValue("name"))
	slug := strings.TrimSpace(r.FormValue("slug"))
	if slug == "" {
		slug = common.Slugify(name)
	}
	description := common.NullString(r.FormValue("description"))
	categoryID := common.NullInt64(r.FormValue("category_id"))

	if errs := validateProduct(name, slug); len(errs) > 0 {
		s.reRenderProductForm(w, r, sqlc.Product{ID: id}, true, errs)
		return
	}

	err = s.Q.UpdateProduct(ctx, sqlc.UpdateProductParams{
		Name:        name,
		Slug:        slug,
		Description: description,
		CategoryID:  categoryID,
		ID:          id,
	})
	if err != nil {
		if common.IsUniqueViolation(err) {
			s.reRenderProductForm(w, r, sqlc.Product{ID: id, Name: name, Slug: slug}, true,
				map[string]string{"slug": "That slug is already taken."})
			return
		}
		common.ServerError(w, err)
		return
	}

	s.SetFlash(r, "Product updated.", "success")
	http.Redirect(w, r, "/products/"+common.Itoa(id), http.StatusSeeOther)
}

// HandleProductArchive handles POST /products/{id}/archive (soft delete).
func (s *Server) HandleProductArchive(w http.ResponseWriter, r *http.Request) {
	s.setProductArchived(w, r, 1)
}

// HandleProductRestore handles POST /products/{id}/restore.
func (s *Server) HandleProductRestore(w http.ResponseWriter, r *http.Request) {
	s.setProductArchived(w, r, 0)
}

func (s *Server) setProductArchived(w http.ResponseWriter, r *http.Request, flag int64) {
	id, err := common.URLID(r, "id")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	archivedAt := sql.NullString{}
	if flag == 1 {
		archivedAt = sql.NullString{String: service.NowUTC(), Valid: true}
	}
	if err := s.Q.SetProductArchived(r.Context(), sqlc.SetProductArchivedParams{
		IsArchived: flag,
		ArchivedAt: archivedAt,
		ID:         id,
	}); err != nil {
		common.ServerError(w, err)
		return
	}
	if flag == 1 {
		s.SetFlash(r, "Product archived.", "warning")
	} else {
		s.SetFlash(r, "Product restored.", "success")
	}
	http.Redirect(w, r, "/products", http.StatusSeeOther)
}

func (s *Server) reRenderProductForm(w http.ResponseWriter, r *http.Request, p sqlc.Product, editing bool, errs map[string]string) {
	categories, err := s.Q.ListCategories(r.Context())
	if err != nil {
		common.ServerError(w, err)
		return
	}
	common.Render(w, r, http.StatusUnprocessableEntity, pages.ProductForm(p, categories, editing, errs, middleware.CSRFToken(r)))
}

func validateProduct(name, slug string) map[string]string {
	errs := map[string]string{}
	if name == "" {
		errs["name"] = "Name is required."
	}
	if slug == "" {
		errs["slug"] = "Slug is required."
	}
	return errs
}
