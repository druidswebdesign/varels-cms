package handlers

import (
	"net/http"
	"strings"

	"github.com/yourname/varels_cms/internal/db/sqlc"
	"github.com/yourname/varels_cms/internal/db/types"
	"github.com/yourname/varels_cms/internal/views/pages"
)

// HandleCollectionsList renders GET /collections.
func (s *Server) HandleCollectionsList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	collections, err := s.Q.ListCollections(ctx, 0)
	if err != nil {
		serverError(w, err)
		return
	}
	render(w, r, http.StatusOK, pages.Collections(collections, CSRFToken(r)))
}

// HandleCollectionCreate handles POST /collections.
func (s *Server) HandleCollectionCreate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	_ = r.ParseForm()

	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		s.setFlash(r, "Collection name is required.", "error")
		http.Redirect(w, r, "/collections", http.StatusSeeOther)
		return
	}
	slug := strings.TrimSpace(r.FormValue("slug"))
	if slug == "" {
		slug = slugify(name)
	}
	kind := types.CollectionKind(strings.TrimSpace(r.FormValue("kind")))
	switch kind {
	case types.CollectionKindDrop, types.CollectionKindSeason, types.CollectionKindEssentials:
	default:
		kind = types.CollectionKindDrop
	}

	if _, err := s.Q.CreateCollection(ctx, sqlc.CreateCollectionParams{
		Name:       name,
		Slug:       slug,
		Kind:       kind,
		Season:     nullString(r.FormValue("season")),
		LaunchDate: nullString(r.FormValue("launch_date")),
	}); err != nil {
		if isUniqueViolation(err) {
			s.setFlash(r, "A collection with that name or slug already exists.", "error")
			http.Redirect(w, r, "/collections", http.StatusSeeOther)
			return
		}
		serverError(w, err)
		return
	}
	s.setFlash(r, "Collection created.", "success")
	http.Redirect(w, r, "/collections", http.StatusSeeOther)
}

// HandleCollectionArchive handles POST /collections/{id}/archive.
func (s *Server) HandleCollectionArchive(w http.ResponseWriter, r *http.Request) {
	id, err := urlID(r, "id")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if err := s.Q.SetCollectionArchived(r.Context(), sqlc.SetCollectionArchivedParams{IsArchived: 1, ID: id}); err != nil {
		serverError(w, err)
		return
	}
	s.setFlash(r, "Collection archived.", "warning")
	http.Redirect(w, r, "/collections", http.StatusSeeOther)
}

// HandleProductCollections handles POST /products/{id}/collections: the posted
// collection_id values become the product's collections, others are removed.
func (s *Server) HandleProductCollections(w http.ResponseWriter, r *http.Request) {
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

	selected := map[int64]bool{}
	for _, raw := range r.Form["collection_id"] {
		if id, ok := parseInt64(raw); ok {
			selected[id] = true
		}
	}

	current, err := s.Q.ListProductCollections(ctx, productID)
	if err != nil {
		serverError(w, err)
		return
	}
	currentIDs := map[int64]bool{}
	for _, c := range current {
		currentIDs[c.ID] = true
		if !selected[c.ID] {
			if err := s.Q.RemoveProductFromCollection(ctx, sqlc.RemoveProductFromCollectionParams{
				ProductID: productID, CollectionID: c.ID,
			}); err != nil {
				serverError(w, err)
				return
			}
		}
	}
	for id := range selected {
		if currentIDs[id] {
			continue
		}
		if err := s.Q.AddProductToCollection(ctx, sqlc.AddProductToCollectionParams{
			ProductID: productID, CollectionID: id,
		}); err != nil {
			serverError(w, err)
			return
		}
	}

	s.audit(r, "update", "product_collections", productID, "collections")
	s.setFlash(r, "Collections updated.", "success")
	http.Redirect(w, r, "/products/"+itoa(productID), http.StatusSeeOther)
}
