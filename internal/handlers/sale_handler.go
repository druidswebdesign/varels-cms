package handlers

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/yourname/varels_cms/internal/db/sqlc"
	"github.com/yourname/varels_cms/internal/db/types"
	"github.com/yourname/varels_cms/internal/views/pages"
	"github.com/yourname/varels_cms/internal/views/partials"
)

const orderListLimit = 100

var (
	errNoLines            = errors.New("no line items")
	errStoreCreditNoCust  = errors.New("store credit needs a customer")
	errInsufficientCredit = errors.New("insufficient store credit")
	errBadReturn          = errors.New("invalid return")
)

// HandleSalesList renders GET /sales with filters.
func (s *Server) HandleSalesList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orders, err := s.listOrdersFromQuery(ctx, r)
	if err != nil {
		serverError(w, err)
		return
	}
	render(w, r, http.StatusOK, pages.SalesList(
		orders,
		s.channelNames(ctx),
		s.locationNamesOrEmpty(ctx),
		s.customerNames(ctx),
		CSRFToken(r),
	))
}

// HandleSalesFilter renders GET /sales/filter as an HTMX fragment: just the
// order rows, for the filter form on the sales index.
func (s *Server) HandleSalesFilter(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orders, err := s.listOrdersFromQuery(ctx, r)
	if err != nil {
		serverError(w, err)
		return
	}
	render(w, r, http.StatusOK, partials.SaleRows(orders, s.channelNames(ctx), s.customerNames(ctx)))
}

// listOrdersFromQuery builds the ListOrders filter from the request query.
// Date bounds are Argentina-local days (ADR-0014).
func (s *Server) listOrdersFromQuery(ctx context.Context, r *http.Request) ([]sqlc.Order, error) {
	query := r.URL.Query()

	var fromTs, toTs, channelID, locationID, status interface{}
	if v := strings.TrimSpace(query.Get("from")); v != "" {
		if t, err := time.ParseInLocation("2006-01-02", v, art); err == nil {
			fromTs = isoUTC(t)
		}
	}
	if v := strings.TrimSpace(query.Get("to")); v != "" {
		if t, err := time.ParseInLocation("2006-01-02", v, art); err == nil {
			toTs = isoUTC(t.AddDate(0, 0, 1).Add(-time.Second))
		}
	}
	if v, ok := parseInt64(query.Get("channel")); ok {
		channelID = v
	}
	if v, ok := parseInt64(query.Get("location")); ok {
		locationID = v
	}
	if v := strings.TrimSpace(query.Get("status")); v != "" {
		status = v
	}

	return s.Q.ListOrders(ctx, sqlc.ListOrdersParams{
		FromTs:     fromTs,
		ToTs:       toTs,
		ChannelID:  channelID,
		LocationID: locationID,
		Status:     status,
		Limit:      orderListLimit,
		Offset:     0,
	})
}

// HandlePaymentCreate handles POST /sales/{id}/payments (split tender).
func (s *Server) HandlePaymentCreate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, err := urlID(r, "id")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	order, err := s.Q.GetOrder(ctx, id)
	if err != nil {
		if isNotFound(err) {
			http.NotFound(w, r)
			return
		}
		serverError(w, err)
		return
	}
	_ = r.ParseForm()

	method := types.PaymentMethod(strings.TrimSpace(r.FormValue("method")))
	if !validPaymentMethod(method) {
		s.setFlash(r, "Invalid payment method.", "error")
		http.Redirect(w, r, "/sales/"+itoa(id), http.StatusSeeOther)
		return
	}
	amount, amountErr := parsePesos(r.FormValue("amount"))
	if amountErr != nil || amount <= 0 {
		s.setFlash(r, "Enter a positive payment amount.", "error")
		http.Redirect(w, r, "/sales/"+itoa(id), http.StatusSeeOther)
		return
	}

	if _, err := s.Q.CreatePayment(ctx, sqlc.CreatePaymentParams{
		OrderID:     id,
		Method:      method,
		AmountMinor: amount,
		Status:      types.PaymentRecordCaptured,
		Reference:   nullString(r.FormValue("reference")),
		PaidAt:      nowUTC(),
		CreatedBy:   currentUserID(r),
	}); err != nil {
		serverError(w, err)
		return
	}

	payments, err := s.Q.ListPaymentsByOrder(ctx, id)
	if err == nil {
		var captured int64
		for _, p := range payments {
			if p.Status == types.PaymentRecordCaptured {
				captured += p.AmountMinor
			}
		}
		if captured >= order.TotalMinor {
			_ = s.Q.SetOrderPaymentStatus(ctx, sqlc.SetOrderPaymentStatusParams{
				PaymentStatus: types.PaymentStatusPaid,
				ID:            id,
			})
		}
	}

	s.audit(r, "payment", "order", id, "split payment")
	s.setFlash(r, "Payment recorded.", "success")
	http.Redirect(w, r, "/sales/"+itoa(id), http.StatusSeeOther)
}

// HandleSaleForm renders the POS at GET /sales/new.
func (s *Server) HandleSaleForm(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	variants, err := s.Q.ListSellableVariants(ctx)
	if err != nil {
		serverError(w, err)
		return
	}
	customers, err := s.Q.ListCustomers(ctx, sqlc.ListCustomersParams{Limit: 500, Offset: 0})
	if err != nil {
		serverError(w, err)
		return
	}
	channels, err := s.Q.ListSalesChannels(ctx, 1)
	if err != nil {
		serverError(w, err)
		return
	}
	locations, err := s.Q.ListLocations(ctx, 1)
	if err != nil {
		serverError(w, err)
		return
	}
	render(w, r, http.StatusOK, pages.SaleForm(variants, customers, channels, locations, CSRFToken(r), r.URL.Query().Get("err")))
}

// HandleSaleCreate handles POST /sales: one atomic transaction that writes the
// order, its items, stock movements, FIFO consumption and the payment
// (ADR-0012, ADR-0019, ADR-0021).
func (s *Server) HandleSaleCreate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	_ = r.ParseForm()

	channelID, chOK := parseInt64(r.FormValue("channel_id"))
	locationID, locOK := parseInt64(r.FormValue("location_id"))
	method := types.PaymentMethod(strings.TrimSpace(r.FormValue("payment_method")))
	if !chOK || !locOK || !validPaymentMethod(method) {
		http.Redirect(w, r, "/sales/new?err=invalid", http.StatusSeeOther)
		return
	}

	orderDiscount, _ := parsePesos(r.FormValue("order_discount"))
	shipping, _ := parsePesos(r.FormValue("shipping"))
	note := strings.TrimSpace(r.FormValue("note"))
	newCustomerName := strings.TrimSpace(r.FormValue("new_customer_name"))
	newCustomerPhone := strings.TrimSpace(r.FormValue("new_customer_phone"))

	var customerID sql.NullInt64
	if v, ok := parseInt64(r.FormValue("customer_id")); ok {
		customerID = sql.NullInt64{Int64: v, Valid: true}
	}

	variantIDs := r.Form["variant_id"]
	quantities := r.Form["quantity"]
	prices := r.Form["unit_price"]
	discounts := r.Form["line_discount"]

	var orderID int64
	err := s.withTx(ctx, func(q *sqlc.Queries) error {
		if newCustomerName != "" && !customerID.Valid {
			c, err := q.CreateCustomer(ctx, sqlc.CreateCustomerParams{
				Name:  newCustomerName,
				Phone: sql.NullString{String: newCustomerPhone, Valid: newCustomerPhone != ""},
			})
			if err != nil {
				return err
			}
			customerID = sql.NullInt64{Int64: c.ID, Valid: true}
		}

		type line struct {
			variantID    int64
			qty          int64
			unitPrice    int64
			lineDiscount int64
			lineTotal    int64
			cost         int64
		}
		var lines []line
		for i, raw := range variantIDs {
			vid, ok := parseInt64(raw)
			if !ok {
				continue
			}
			var qty int64
			if i < len(quantities) {
				qty, _ = parseInt64(quantities[i])
			}
			if qty <= 0 {
				continue
			}
			v, err := q.GetVariant(ctx, vid)
			if err != nil {
				return err
			}
			unitPrice := v.RetailPriceMinor
			if i < len(prices) {
				if p, err := parsePesos(prices[i]); err == nil && p > 0 {
					unitPrice = p
				}
			}
			var lineDiscount int64
			if i < len(discounts) {
				lineDiscount, _ = parsePesos(discounts[i])
			}
			gross := unitPrice * qty
			if lineDiscount > gross {
				lineDiscount = gross
			}
			cost, err := consumeFIFO(ctx, q, vid, qty, v.CostMinor)
			if err != nil {
				return err
			}
			lines = append(lines, line{
				variantID:    vid,
				qty:          qty,
				unitPrice:    unitPrice,
				lineDiscount: lineDiscount,
				lineTotal:    gross - lineDiscount,
				cost:         cost,
			})
		}
		if len(lines) == 0 {
			return errNoLines
		}

		var subtotal int64
		for _, l := range lines {
			subtotal += l.lineTotal
		}
		if orderDiscount > subtotal {
			orderDiscount = subtotal
		}
		total := subtotal - orderDiscount + shipping
		if total < 0 {
			total = 0
		}

		order, err := q.CreateOrder(ctx, sqlc.CreateOrderParams{
			OrderNumber:   newOrderNumber(),
			CustomerID:    customerID,
			ChannelID:     channelID,
			LocationID:    locationID,
			Status:        types.OrderStatusCompleted,
			SubtotalMinor: subtotal,
			DiscountMinor: orderDiscount,
			TaxMinor:      ivaTaxMinor(total),
			ShippingMinor: shipping,
			TotalMinor:    total,
			PaymentStatus: types.PaymentStatusPaid,
			PlacedAt:      nowUTC(),
			Note:          sql.NullString{String: note, Valid: note != ""},
			CreatedBy:     currentUserID(r),
		})
		if err != nil {
			return err
		}
		orderID = order.ID

		for _, l := range lines {
			if _, err := q.CreateOrderItem(ctx, sqlc.CreateOrderItemParams{
				OrderID:        order.ID,
				VariantID:      l.variantID,
				Quantity:       l.qty,
				UnitPriceMinor: l.unitPrice,
				UnitCostMinor:  l.cost,
				DiscountMinor:  l.lineDiscount,
				LineTotalMinor: l.lineTotal,
			}); err != nil {
				return err
			}
			if _, err := q.CreateStockMovement(ctx, sqlc.CreateStockMovementParams{
				VariantID:     l.variantID,
				LocationID:    locationID,
				State:         types.StockStateSellable,
				QuantityDelta: -l.qty,
				RefType:       types.StockRefOrder,
				RefID:         sql.NullInt64{Int64: order.ID, Valid: true},
				CreatedBy:     currentUserID(r),
			}); err != nil {
				return err
			}
		}

		if total > 0 {
			if method == types.PaymentMethodStoreCredit {
				if !customerID.Valid {
					return errStoreCreditNoCust
				}
				balance := asInt64Or(q.StoreCreditBalance(ctx, customerID.Int64))
				if balance < total {
					return errInsufficientCredit
				}
				if _, err := q.CreateStoreCreditEntry(ctx, sqlc.CreateStoreCreditEntryParams{
					CustomerID: customerID.Int64,
					DeltaMinor: -total,
					Reason:     types.StoreCreditReasonRedeemed,
					OrderID:    sql.NullInt64{Int64: order.ID, Valid: true},
					Note:       sql.NullString{String: order.OrderNumber, Valid: true},
					CreatedBy:  currentUserID(r),
				}); err != nil {
					return err
				}
			}
			if _, err := q.CreatePayment(ctx, sqlc.CreatePaymentParams{
				OrderID:     order.ID,
				Method:      method,
				AmountMinor: total,
				Status:      types.PaymentRecordCaptured,
				PaidAt:      nowUTC(),
				CreatedBy:   currentUserID(r),
			}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		switch {
		case errors.Is(err, errNoLines):
			http.Redirect(w, r, "/sales/new?err=lines", http.StatusSeeOther)
		case errors.Is(err, errStoreCreditNoCust):
			http.Redirect(w, r, "/sales/new?err=credit_customer", http.StatusSeeOther)
		case errors.Is(err, errInsufficientCredit):
			http.Redirect(w, r, "/sales/new?err=credit_balance", http.StatusSeeOther)
		case strings.Contains(err.Error(), "constraint failed"):
			// The stock trigger's CHECK (quantity >= 0) rejected an oversell.
			http.Redirect(w, r, "/sales/new?err=stock", http.StatusSeeOther)
		default:
			serverError(w, err)
		}
		return
	}

	s.audit(r, "create", "order", orderID, "sale")
	s.setFlash(r, "Sale completed.", "success")
	http.Redirect(w, r, "/sales/"+itoa(orderID), http.StatusSeeOther)
}

// HandleSaleDetail renders the receipt/detail page.
func (s *Server) HandleSaleDetail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, err := urlID(r, "id")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	data, err := s.saleDetail(ctx, id)
	if err != nil {
		if isNotFound(err) {
			http.NotFound(w, r)
			return
		}
		serverError(w, err)
		return
	}
	render(w, r, http.StatusOK, pages.SaleDetail(data, CSRFToken(r)))
}

// HandleSaleVoid reverses a sale: restock, compensate the payment and mark the
// order cancelled (never delete it).
func (s *Server) HandleSaleVoid(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, err := urlID(r, "id")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	order, err := s.Q.GetOrder(ctx, id)
	if err != nil {
		if isNotFound(err) {
			http.NotFound(w, r)
			return
		}
		serverError(w, err)
		return
	}
	if order.Status == types.OrderStatusCancelled {
		http.Redirect(w, r, "/sales/"+itoa(id), http.StatusSeeOther)
		return
	}

	err = s.withTx(ctx, func(q *sqlc.Queries) error {
		items, err := q.ListOrderItems(ctx, id)
		if err != nil {
			return err
		}
		for _, it := range items {
			if _, err := q.CreateStockMovement(ctx, sqlc.CreateStockMovementParams{
				VariantID:     it.VariantID,
				LocationID:    order.LocationID,
				State:         types.StockStateSellable,
				QuantityDelta: it.Quantity,
				RefType:       types.StockRefAdjustment,
				ReasonCode:    sql.NullString{String: "recount", Valid: true},
				Note:          sql.NullString{String: "void " + order.OrderNumber, Valid: true},
				CreatedBy:     currentUserID(r),
			}); err != nil {
				return err
			}
			if err := addCostLayer(ctx, q, it.VariantID, it.Quantity, it.UnitCostMinor, "void:"+order.OrderNumber); err != nil {
				return err
			}
		}
		payments, err := q.ListPaymentsByOrder(ctx, id)
		if err != nil {
			return err
		}
		for _, p := range payments {
			if err := q.SetPaymentStatus(ctx, sqlc.SetPaymentStatusParams{
				Status: types.PaymentRecordRefunded,
				ID:     p.ID,
			}); err != nil {
				return err
			}
		}
		if err := q.SetOrderPaymentStatus(ctx, sqlc.SetOrderPaymentStatusParams{
			PaymentStatus: types.PaymentStatusRefunded,
			ID:            id,
		}); err != nil {
			return err
		}
		return q.SetOrderStatus(ctx, sqlc.SetOrderStatusParams{
			Status: types.OrderStatusCancelled,
			ID:     id,
		})
	})
	if err != nil {
		serverError(w, err)
		return
	}
	s.audit(r, "void", "order", id, "sale voided")
	s.setFlash(r, "Sale voided and stock restored.", "warning")
	http.Redirect(w, r, "/sales/"+itoa(id), http.StatusSeeOther)
}

// HandleReturnForm renders the partial-return form.
func (s *Server) HandleReturnForm(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, err := urlID(r, "id")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	data, err := s.saleDetail(ctx, id)
	if err != nil {
		if isNotFound(err) {
			http.NotFound(w, r)
			return
		}
		serverError(w, err)
		return
	}
	reasons, err := s.Q.ListReasonCodes(ctx, types.ReasonKindReturn)
	if err != nil {
		serverError(w, err)
		return
	}
	render(w, r, http.StatusOK, pages.ReturnForm(data, reasons, CSRFToken(r)))
}

// HandleReturnCreate handles POST /sales/{id}/return: per line quantity,
// condition and reason, restocking to sellable or damaged and issuing refund or
// store credit (ADR-0006, ADR-0020).
func (s *Server) HandleReturnCreate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, err := urlID(r, "id")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	_ = r.ParseForm()

	order, err := s.Q.GetOrder(ctx, id)
	if err != nil {
		if isNotFound(err) {
			http.NotFound(w, r)
			return
		}
		serverError(w, err)
		return
	}

	resolution := types.ReturnResolution(strings.TrimSpace(r.FormValue("resolution")))
	if resolution != types.ReturnResolutionRefund &&
		resolution != types.ReturnResolutionStoreCredit &&
		resolution != types.ReturnResolutionExchange {
		http.Redirect(w, r, "/sales/"+itoa(id)+"/return?err=resolution", http.StatusSeeOther)
		return
	}
	note := strings.TrimSpace(r.FormValue("note"))

	itemIDs := r.Form["item_id"]
	qtys := r.Form["qty"]
	conditions := r.Form["condition"]
	reasons := r.Form["reason"]

	err = s.withTx(ctx, func(q *sqlc.Queries) error {
		items, err := q.ListOrderItems(ctx, id)
		if err != nil {
			return err
		}
		byID := make(map[int64]sqlc.OrderItem, len(items))
		for _, it := range items {
			byID[it.ID] = it
		}

		type retLine struct {
			item      sqlc.OrderItem
			qty       int64
			condition types.ReturnCondition
			reason    string
			refund    int64
		}
		var lines []retLine
		var totalRefund int64
		for i, raw := range itemIDs {
			itemID, ok := parseInt64(raw)
			if !ok {
				continue
			}
			var qty int64
			if i < len(qtys) {
				qty, _ = parseInt64(qtys[i])
			}
			if qty <= 0 {
				continue
			}
			item, ok := byID[itemID]
			if !ok {
				return errBadReturn
			}
			already := asInt64Or(q.ReturnedQuantityByOrderItem(ctx, itemID))
			if qty > item.Quantity-already {
				return errBadReturn
			}
			cond := types.ReturnCondition("sellable")
			if i < len(conditions) && conditions[i] == "damaged" {
				cond = types.ReturnConditionDamaged
			}
			reason := "changed_mind"
			if i < len(reasons) && strings.TrimSpace(reasons[i]) != "" {
				reason = strings.TrimSpace(reasons[i])
			}
			refund := item.UnitPriceMinor * qty
			totalRefund += refund
			lines = append(lines, retLine{item: item, qty: qty, condition: cond, reason: reason, refund: refund})
		}
		if len(lines) == 0 {
			return errBadReturn
		}
		if resolution == types.ReturnResolutionStoreCredit && !order.CustomerID.Valid {
			return errStoreCreditNoCust
		}

		ret, err := q.CreateReturn(ctx, sqlc.CreateReturnParams{
			OrderID:          id,
			CustomerID:       order.CustomerID,
			Resolution:       resolution,
			Status:           types.ReturnStatusCompleted,
			TotalRefundMinor: totalRefund,
			Note:             sql.NullString{String: note, Valid: note != ""},
			ProcessedAt:      sql.NullString{String: nowUTC(), Valid: true},
			CreatedBy:        currentUserID(r),
		})
		if err != nil {
			return err
		}

		for _, l := range lines {
			if _, err := q.CreateReturnItem(ctx, sqlc.CreateReturnItemParams{
				ReturnID:          ret.ID,
				OrderItemID:       l.item.ID,
				Quantity:          l.qty,
				ConditionState:    l.condition,
				Resolution:        resolution,
				RefundAmountMinor: l.refund,
				ReasonCode:        l.reason,
			}); err != nil {
				return err
			}
			state := types.StockStateSellable
			if l.condition == types.ReturnConditionDamaged {
				state = types.StockStateDamaged
			}
			if _, err := q.CreateStockMovement(ctx, sqlc.CreateStockMovementParams{
				VariantID:     l.item.VariantID,
				LocationID:    order.LocationID,
				State:         state,
				QuantityDelta: l.qty,
				ReasonCode:    sql.NullString{String: l.reason, Valid: true},
				RefType:       types.StockRefReturn,
				RefID:         sql.NullInt64{Int64: ret.ID, Valid: true},
				CreatedBy:     currentUserID(r),
			}); err != nil {
				return err
			}
			if err := addCostLayer(ctx, q, l.item.VariantID, l.qty, l.item.UnitCostMinor, "return:"+itoa(ret.ID)); err != nil {
				return err
			}
		}

		if resolution == types.ReturnResolutionStoreCredit {
			if _, err := q.CreateStoreCreditEntry(ctx, sqlc.CreateStoreCreditEntryParams{
				CustomerID: order.CustomerID.Int64,
				DeltaMinor: totalRefund,
				Reason:     types.StoreCreditReasonIssued,
				OrderID:    sql.NullInt64{Int64: id, Valid: true},
				ReturnID:   sql.NullInt64{Int64: ret.ID, Valid: true},
				Note:       sql.NullString{String: "return " + order.OrderNumber, Valid: true},
				CreatedBy:  currentUserID(r),
			}); err != nil {
				return err
			}
		}

		return updateOrderAfterReturn(ctx, q, id, order, resolution)
	})
	if err != nil {
		switch {
		case errors.Is(err, errBadReturn):
			http.Redirect(w, r, "/sales/"+itoa(id)+"/return?err=quantity", http.StatusSeeOther)
		case errors.Is(err, errStoreCreditNoCust):
			http.Redirect(w, r, "/sales/"+itoa(id)+"/return?err=credit_customer", http.StatusSeeOther)
		default:
			serverError(w, err)
		}
		return
	}

	s.audit(r, "return", "order", id, "return processed")
	s.setFlash(r, "Return processed.", "success")
	http.Redirect(w, r, "/sales/"+itoa(id), http.StatusSeeOther)
}

// updateOrderAfterReturn recomputes order status and payment status from the
// cumulative returned quantities.
func updateOrderAfterReturn(ctx context.Context, q *sqlc.Queries, orderID int64, order sqlc.Order, resolution types.ReturnResolution) error {
	items, err := q.ListOrderItems(ctx, orderID)
	if err != nil {
		return err
	}
	var ordered, returned int64
	for _, it := range items {
		ordered += it.Quantity
		returned += asInt64Or(q.ReturnedQuantityByOrderItem(ctx, it.ID))
	}

	status := types.OrderStatusPartiallyReturned
	if returned >= ordered {
		status = types.OrderStatusReturned
	}
	if err := q.SetOrderStatus(ctx, sqlc.SetOrderStatusParams{Status: status, ID: orderID}); err != nil {
		return err
	}
	if resolution == types.ReturnResolutionRefund {
		paymentStatus := types.PaymentStatusPartiallyRefunded
		if returned >= ordered {
			paymentStatus = types.PaymentStatusRefunded
		}
		if err := q.SetOrderPaymentStatus(ctx, sqlc.SetOrderPaymentStatusParams{PaymentStatus: paymentStatus, ID: orderID}); err != nil {
			return err
		}
	}
	return nil
}

// saleDetail assembles the receipt/detail view model.
func (s *Server) saleDetail(ctx context.Context, id int64) (pages.SaleDetailData, error) {
	order, err := s.Q.GetOrder(ctx, id)
	if err != nil {
		return pages.SaleDetailData{}, err
	}
	items, err := s.Q.ListOrderItems(ctx, id)
	if err != nil {
		return pages.SaleDetailData{}, err
	}
	payments, err := s.Q.ListPaymentsByOrder(ctx, id)
	if err != nil {
		return pages.SaleDetailData{}, err
	}
	returns, err := s.Q.ListReturnsByOrder(ctx, id)
	if err != nil {
		return pages.SaleDetailData{}, err
	}

	variants, _ := s.Q.ListSellableVariants(ctx)
	variantByID := make(map[int64]sqlc.ListSellableVariantsRow, len(variants))
	for _, v := range variants {
		variantByID[v.ID] = v
	}

	lines := make([]pages.SaleLine, 0, len(items))
	for _, it := range items {
		v := variantByID[it.VariantID]
		returned := asInt64Or(s.Q.ReturnedQuantityByOrderItem(ctx, it.ID))
		lines = append(lines, pages.SaleLine{
			Item:        it,
			Sku:         v.Sku,
			ProductName: v.ProductName,
			Size:        v.Size.String,
			Color:       v.Color.String,
			ReturnedQty: returned,
			Returnable:  it.Quantity - returned,
		})
	}

	customer := "—"
	if order.CustomerID.Valid {
		if c, err := s.Q.GetCustomer(ctx, order.CustomerID.Int64); err == nil {
			customer = c.Name
		}
	}

	data := pages.SaleDetailData{
		Order:       order,
		Customer:    customer,
		Channel:     s.channelNames(ctx)[order.ChannelID],
		Location:    s.locationNamesOrEmpty(ctx)[order.LocationID],
		Lines:       lines,
		Payments:    payments,
		Returns:     returns,
		HasCustomer: order.CustomerID.Valid,
	}
	if order.CustomerID.Valid {
		data.StoreCredit = asInt64Or(s.Q.StoreCreditBalance(ctx, order.CustomerID.Int64))
	}
	return data, nil
}

func (s *Server) channelNames(ctx context.Context) map[int64]string {
	channels, err := s.Q.ListSalesChannels(ctx, 1)
	out := make(map[int64]string)
	if err != nil {
		return out
	}
	for _, c := range channels {
		out[c.ID] = c.Name
	}
	return out
}

func (s *Server) customerNames(ctx context.Context) map[int64]string {
	customers, err := s.Q.ListCustomers(ctx, sqlc.ListCustomersParams{Limit: 1000, Offset: 0})
	out := make(map[int64]string)
	if err != nil {
		return out
	}
	for _, c := range customers {
		out[c.ID] = c.Name
	}
	return out
}

func (s *Server) locationNamesOrEmpty(ctx context.Context) map[int64]string {
	out, err := s.locationNames(ctx)
	if err != nil {
		return map[int64]string{}
	}
	return out
}

func validPaymentMethod(m types.PaymentMethod) bool {
	switch m {
	case types.PaymentMethodCash, types.PaymentMethodCard, types.PaymentMethodMercadoPago,
		types.PaymentMethodTransfer, types.PaymentMethodStoreCredit:
		return true
	}
	return false
}

// asInt64Or wraps an (interface{}, error) aggregate result.
func asInt64Or(v interface{}, _ error) int64 {
	return asInt64(v)
}
