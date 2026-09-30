package pages

import "github.com/druidswebdesign/varels-cms/internal/db/sqlc"

// SaleLine is one order line enriched for rendering (receipt/detail/return).
type SaleLine struct {
	Item        sqlc.OrderItem
	Sku         string
	ProductName string
	Size        string
	Color       string
	ReturnedQty int64
	Returnable  int64
}

// SaleDetailData is the receipt/detail/return view model.
type SaleDetailData struct {
	Order       sqlc.Order
	Customer    string
	Channel     string
	Location    string
	Lines       []SaleLine
	Payments    []sqlc.Payment
	Returns     []sqlc.Return
	StoreCredit int64
	HasCustomer bool
}
