package handlers

import (
	"net/http"
	"strings"
	"time"

	"github.com/yourname/varels_cms/internal/db/sqlc"
	"github.com/yourname/varels_cms/internal/views/pages"
)

const expenseListLimit = 200

// HandleExpensesList renders GET /expenses (admin+).
func (s *Server) HandleExpensesList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	expenses, err := s.Q.ListExpenses(ctx, sqlc.ListExpensesParams{Limit: expenseListLimit, Offset: 0})
	if err != nil {
		serverError(w, err)
		return
	}
	categories, err := s.Q.ListExpenseCategories(ctx, 1)
	if err != nil {
		serverError(w, err)
		return
	}
	var total int64
	for _, e := range expenses {
		total += e.AmountMinor
	}
	render(w, r, http.StatusOK, pages.Expenses(expenses, categories, total, CSRFToken(r)))
}

// HandleExpenseCreate handles POST /expenses. The date is interpreted in
// Argentina-local time and stored as UTC ISO (ADR-0014).
func (s *Server) HandleExpenseCreate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	_ = r.ParseForm()

	categoryID, catOK := parseInt64(r.FormValue("category_id"))
	amount, amountErr := parsePesos(r.FormValue("amount"))
	dateStr := strings.TrimSpace(r.FormValue("incurred_at"))
	if !catOK || amountErr != nil || amount <= 0 || dateStr == "" {
		s.setFlash(r, "Expense needs a category, a positive amount and a date.", "error")
		http.Redirect(w, r, "/expenses", http.StatusSeeOther)
		return
	}

	incurred := dateStr
	if t, err := time.ParseInLocation("2006-01-02", dateStr, art); err == nil {
		incurred = isoUTC(t)
	}

	expense, err := s.Q.CreateExpense(ctx, sqlc.CreateExpenseParams{
		CategoryID:  categoryID,
		Vendor:      nullString(r.FormValue("vendor")),
		AmountMinor: amount,
		IncurredAt:  incurred,
		Note:        nullString(r.FormValue("note")),
		CreatedBy:   currentUserID(r),
	})
	if err != nil {
		serverError(w, err)
		return
	}

	s.audit(r, "create", "expense", expense.ID, "opex")
	s.setFlash(r, "Expense recorded.", "success")
	http.Redirect(w, r, "/expenses", http.StatusSeeOther)
}

// HandleExpenseCategories renders GET /expense-categories.
func (s *Server) HandleExpenseCategories(w http.ResponseWriter, r *http.Request) {
	categories, err := s.Q.ListExpenseCategories(r.Context(), 1)
	if err != nil {
		serverError(w, err)
		return
	}
	render(w, r, http.StatusOK, pages.ExpenseCategories(categories, CSRFToken(r)))
}

// HandleExpenseCategoryCreate handles POST /expense-categories.
func (s *Server) HandleExpenseCategoryCreate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	_ = r.ParseForm()

	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		s.setFlash(r, "Category name is required.", "error")
		http.Redirect(w, r, "/expense-categories", http.StatusSeeOther)
		return
	}
	if _, err := s.Q.CreateExpenseCategory(ctx, name); err != nil {
		if isUniqueViolation(err) {
			s.setFlash(r, "That category already exists.", "error")
			http.Redirect(w, r, "/expense-categories", http.StatusSeeOther)
			return
		}
		serverError(w, err)
		return
	}
	s.setFlash(r, "Category added.", "success")
	http.Redirect(w, r, "/expense-categories", http.StatusSeeOther)
}
