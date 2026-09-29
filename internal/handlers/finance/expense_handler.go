package finance

import (
	"net/http"
	"strings"
	"time"

	"github.com/yourname/varels_cms/internal/db/sqlc"
	"github.com/yourname/varels_cms/internal/handlers/common"
	"github.com/yourname/varels_cms/internal/handlers/middleware"
	"github.com/yourname/varels_cms/internal/reporting"
	"github.com/yourname/varels_cms/internal/views/pages"
)

const expenseListLimit = 200

// HandleExpensesList renders GET /expenses (admin+).
func (s *Server) HandleExpensesList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	expenses, err := s.Q.ListExpenses(ctx, sqlc.ListExpensesParams{Limit: expenseListLimit, Offset: 0})
	if err != nil {
		common.ServerError(w, err)
		return
	}
	categories, err := s.Q.ListExpenseCategories(ctx, 1)
	if err != nil {
		common.ServerError(w, err)
		return
	}
	var total int64
	for _, e := range expenses {
		total += e.AmountMinor
	}
	common.Render(w, r, http.StatusOK, pages.Expenses(expenses, categories, total, middleware.CSRFToken(r)))
}

// HandleExpenseCreate handles POST /expenses. The date is interpreted in
// Argentina-local time and stored as UTC ISO (ADR-0014).
func (s *Server) HandleExpenseCreate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	_ = r.ParseForm()

	categoryID, catOK := common.ParseInt64(r.FormValue("category_id"))
	amount, amountErr := common.ParsePesos(r.FormValue("amount"))
	dateStr := strings.TrimSpace(r.FormValue("incurred_at"))
	if !catOK || amountErr != nil || amount <= 0 || dateStr == "" {
		s.SetFlash(r, "Expense needs a category, a positive amount and a date.", "error")
		http.Redirect(w, r, "/expenses", http.StatusSeeOther)
		return
	}

	incurred := dateStr
	if t, err := time.ParseInLocation("2006-01-02", dateStr, reporting.ART); err == nil {
		incurred = reporting.ISOUTC(t)
	}

	expense, err := s.Q.CreateExpense(ctx, sqlc.CreateExpenseParams{
		CategoryID:  categoryID,
		Vendor:      common.NullString(r.FormValue("vendor")),
		AmountMinor: amount,
		IncurredAt:  incurred,
		Note:        common.NullString(r.FormValue("note")),
		CreatedBy:   common.CurrentUserID(r),
	})
	if err != nil {
		common.ServerError(w, err)
		return
	}

	s.Audit(r, "create", "expense", expense.ID, "opex")
	s.SetFlash(r, "Expense recorded.", "success")
	http.Redirect(w, r, "/expenses", http.StatusSeeOther)
}

// HandleExpenseCategories renders GET /expense-categories.
func (s *Server) HandleExpenseCategories(w http.ResponseWriter, r *http.Request) {
	categories, err := s.Q.ListExpenseCategories(r.Context(), 1)
	if err != nil {
		common.ServerError(w, err)
		return
	}
	common.Render(w, r, http.StatusOK, pages.ExpenseCategories(categories, middleware.CSRFToken(r)))
}

// HandleExpenseCategoryCreate handles POST /expense-categories.
func (s *Server) HandleExpenseCategoryCreate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	_ = r.ParseForm()

	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		s.SetFlash(r, "Category name is required.", "error")
		http.Redirect(w, r, "/expense-categories", http.StatusSeeOther)
		return
	}
	if _, err := s.Q.CreateExpenseCategory(ctx, name); err != nil {
		if common.IsUniqueViolation(err) {
			s.SetFlash(r, "That category already exists.", "error")
			http.Redirect(w, r, "/expense-categories", http.StatusSeeOther)
			return
		}
		common.ServerError(w, err)
		return
	}
	s.SetFlash(r, "Category added.", "success")
	http.Redirect(w, r, "/expense-categories", http.StatusSeeOther)
}
