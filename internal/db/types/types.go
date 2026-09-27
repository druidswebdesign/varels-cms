// Package types holds the named Go string types that map to the CHECK-constrained
// TEXT columns in the schema (see adr.md and docs/schema.md §8). sqlc overrides in
// sqlc.yaml point the enum columns at these types instead of bare string.
package types

// UserRole is approved_users.role (ADR-0018).
type UserRole string

const (
	UserRoleAdmin UserRole = "admin"
	UserRoleStaff UserRole = "staff"
)

// AuthProvider is approved_users.provider.
type AuthProvider string

const (
	AuthProviderGoogle AuthProvider = "google"
	AuthProviderLocal  AuthProvider = "local"
)

// StockState is stock_levels.state / stock_movements.state (ADR-0009).
type StockState string

const (
	StockStateSellable StockState = "sellable"
	StockStateReserved StockState = "reserved"
	StockStateDamaged  StockState = "damaged"
	StockStateSample   StockState = "sample"
	StockStateGift     StockState = "gift"
	StockStatePersonal StockState = "personal"
)

// StockRefType is stock_movements.ref_type.
type StockRefType string

const (
	StockRefOrder      StockRefType = "order"
	StockRefReturn     StockRefType = "return"
	StockRefAdjustment StockRefType = "adjustment"
	StockRefPOReceipt  StockRefType = "po_receipt"
	StockRefTransfer   StockRefType = "transfer"
	StockRefHold       StockRefType = "hold"
)

// HoldSource is stock_holds.source.
type HoldSource string

const (
	HoldSourceCart     HoldSource = "cart"
	HoldSourcePreorder HoldSource = "preorder"
	HoldSourceManual   HoldSource = "manual"
)

// HoldStatus is stock_holds.status.
type HoldStatus string

const (
	HoldStatusActive   HoldStatus = "active"
	HoldStatusReleased HoldStatus = "released"
	HoldStatusConsumed HoldStatus = "consumed"
	HoldStatusExpired  HoldStatus = "expired"
)

// ReasonKind is reason_codes.kind.
type ReasonKind string

const (
	ReasonKindStockAdjustment ReasonKind = "stock_adjustment"
	ReasonKindReturn          ReasonKind = "return"
)

// CostSource is cost_history.source.
type CostSource string

const (
	CostSourceManual    CostSource = "manual"
	CostSourcePOReceipt CostSource = "po_receipt"
)

// MediaType is product_media.media_type.
type MediaType string

const (
	MediaTypeImage     MediaType = "image"
	MediaTypeSizeChart MediaType = "size_chart"
	MediaTypeVideo     MediaType = "video"
)

// CollectionKind is collections.kind.
type CollectionKind string

const (
	CollectionKindDrop       CollectionKind = "drop"
	CollectionKindSeason     CollectionKind = "season"
	CollectionKindEssentials CollectionKind = "essentials"
)

// LocationKind is locations.kind.
type LocationKind string

const (
	LocationKindWarehouse  LocationKind = "warehouse"
	LocationKindStorefront LocationKind = "storefront"
)

// DiscountScope is discounts.scope.
type DiscountScope string

const (
	DiscountScopeProduct    DiscountScope = "product"
	DiscountScopeVariant    DiscountScope = "variant"
	DiscountScopeCollection DiscountScope = "collection"
	DiscountScopeOrder      DiscountScope = "order"
)

// DiscountType is discounts.type. Percent values are integer basis points
// (2000 = 20.00%) per ADR-0016.
type DiscountType string

const (
	DiscountTypePercent DiscountType = "percent"
	DiscountTypeAmount  DiscountType = "amount"
)

// OrderStatus is orders.status.
type OrderStatus string

const (
	OrderStatusDraft             OrderStatus = "draft"
	OrderStatusCompleted         OrderStatus = "completed"
	OrderStatusPartiallyReturned OrderStatus = "partially_returned"
	OrderStatusReturned          OrderStatus = "returned"
	OrderStatusCancelled         OrderStatus = "cancelled"
)

// PaymentStatus is orders.payment_status.
type PaymentStatus string

const (
	PaymentStatusUnpaid            PaymentStatus = "unpaid"
	PaymentStatusPaid              PaymentStatus = "paid"
	PaymentStatusPartiallyRefunded PaymentStatus = "partially_refunded"
	PaymentStatusRefunded          PaymentStatus = "refunded"
)

// PaymentMethod is payments.method.
type PaymentMethod string

const (
	PaymentMethodCash        PaymentMethod = "cash"
	PaymentMethodCard        PaymentMethod = "card"
	PaymentMethodMercadoPago PaymentMethod = "mercadopago"
	PaymentMethodTransfer    PaymentMethod = "transfer"
	PaymentMethodStoreCredit PaymentMethod = "store_credit"
)

// PaymentRecordStatus is payments.status.
type PaymentRecordStatus string

const (
	PaymentRecordCaptured      PaymentRecordStatus = "captured"
	PaymentRecordRefunded      PaymentRecordStatus = "refunded"
	PaymentRecordPartialRefund PaymentRecordStatus = "partial_refund"
)

// ReturnResolution is returns.resolution / return_items.resolution.
type ReturnResolution string

const (
	ReturnResolutionRefund      ReturnResolution = "refund"
	ReturnResolutionStoreCredit ReturnResolution = "store_credit"
	ReturnResolutionExchange    ReturnResolution = "exchange"
)

// ReturnStatus is returns.status.
type ReturnStatus string

const (
	ReturnStatusPending   ReturnStatus = "pending"
	ReturnStatusCompleted ReturnStatus = "completed"
)

// ReturnCondition is return_items.condition_state.
type ReturnCondition string

const (
	ReturnConditionSellable ReturnCondition = "sellable"
	ReturnConditionDamaged  ReturnCondition = "damaged"
)

// StoreCreditReason is store_credit_ledger.reason (ADR-0020).
type StoreCreditReason string

const (
	StoreCreditReasonIssued   StoreCreditReason = "issued"
	StoreCreditReasonRedeemed StoreCreditReason = "redeemed"
)

// PurchaseOrderStatus is purchase_orders.status.
type PurchaseOrderStatus string

const (
	PurchaseOrderStatusDraft     PurchaseOrderStatus = "draft"
	PurchaseOrderStatusOrdered   PurchaseOrderStatus = "ordered"
	PurchaseOrderStatusPartial   PurchaseOrderStatus = "partial"
	PurchaseOrderStatusReceived  PurchaseOrderStatus = "received"
	PurchaseOrderStatusCancelled PurchaseOrderStatus = "cancelled"
)
