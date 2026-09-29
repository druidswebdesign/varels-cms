package sales

import (
	"database/sql"
	"net/http"
	"strings"

	"github.com/yourname/varels_cms/internal/db/sqlc"
	"github.com/yourname/varels_cms/internal/handlers/common"
	"github.com/yourname/varels_cms/internal/handlers/middleware"
	"github.com/yourname/varels_cms/internal/repository"
	"github.com/yourname/varels_cms/internal/views/pages"
)

const customerListLimit = 200

// HandleCustomersList renders GET /customers with optional search.
func (s *Server) HandleCustomersList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := strings.TrimSpace(r.URL.Query().Get("q"))

	var customers []sqlc.Customer
	var err error
	if q != "" {
		customers, err = s.Q.SearchCustomers(ctx, sqlc.SearchCustomersParams{
			Q:     "%" + q + "%",
			Limit: customerListLimit,
		})
	} else {
		customers, err = s.Q.ListCustomers(ctx, sqlc.ListCustomersParams{
			Limit:  customerListLimit,
			Offset: 0,
		})
	}
	if err != nil {
		common.ServerError(w, err)
		return
	}
	common.Render(w, r, http.StatusOK, pages.Customers(customers, q, middleware.CSRFToken(r)))
}

// HandleCustomerCreate handles POST /customers.
func (s *Server) HandleCustomerCreate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	_ = r.ParseForm()

	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		s.SetFlash(r, "Customer name is required.", "error")
		http.Redirect(w, r, "/customers", http.StatusSeeOther)
		return
	}
	c, err := s.Q.CreateCustomer(ctx, sqlc.CreateCustomerParams{
		Name:      name,
		Phone:     common.NullString(r.FormValue("phone")),
		Email:     common.NullString(r.FormValue("email")),
		Instagram: common.NullString(r.FormValue("instagram")),
		Notes:     common.NullString(r.FormValue("notes")),
	})
	if err != nil {
		common.ServerError(w, err)
		return
	}
	s.SetFlash(r, "Customer added.", "success")
	http.Redirect(w, r, "/customers/"+common.Itoa(c.ID), http.StatusSeeOther)
}

// HandleCustomerDetail renders GET /customers/{id} with order history and store
// credit.
func (s *Server) HandleCustomerDetail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, err := common.URLID(r, "id")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	c, err := s.Q.GetCustomer(ctx, id)
	if err != nil {
		if common.IsNotFound(err) {
			http.NotFound(w, r)
			return
		}
		common.ServerError(w, err)
		return
	}
	orders, err := s.Q.ListOrdersByCustomer(ctx, sqlc.ListOrdersByCustomerParams{
		CustomerID: sql.NullInt64{Int64: id, Valid: true},
		Limit:      100,
	})
	if err != nil {
		common.ServerError(w, err)
		return
	}
	balanceRaw, err := s.Q.StoreCreditBalance(ctx, id)
	if err != nil {
		common.ServerError(w, err)
		return
	}
	entries, err := s.Q.ListStoreCreditEntries(ctx, id)
	if err != nil {
		common.ServerError(w, err)
		return
	}

	common.Render(w, r, http.StatusOK, pages.CustomerDetail(c, orders, repository.AsInt64(balanceRaw), entries, middleware.CSRFToken(r)))
}

// HandleCustomerUpdate handles POST /customers/{id}.
func (s *Server) HandleCustomerUpdate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, err := common.URLID(r, "id")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	_ = r.ParseForm()

	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		s.SetFlash(r, "Customer name is required.", "error")
		http.Redirect(w, r, "/customers/"+common.Itoa(id), http.StatusSeeOther)
		return
	}
	if err := s.Q.UpdateCustomer(ctx, sqlc.UpdateCustomerParams{
		Name:      name,
		Phone:     common.NullString(r.FormValue("phone")),
		Email:     common.NullString(r.FormValue("email")),
		Instagram: common.NullString(r.FormValue("instagram")),
		Notes:     common.NullString(r.FormValue("notes")),
		ID:        id,
	}); err != nil {
		common.ServerError(w, err)
		return
	}
	s.SetFlash(r, "Customer updated.", "success")
	http.Redirect(w, r, "/customers/"+common.Itoa(id), http.StatusSeeOther)
}
