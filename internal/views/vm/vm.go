// Package vm holds view models that normalise sqlc's aggregate column types
// (interface{} / sql.NullFloat64 from SUM/COALESCE) into plain int64 minor
// units before they reach the templates.
package vm

// BestSeller is one row of the best-sellers report.
type BestSeller struct {
	VariantID    int64
	Sku          string
	ProductName  string
	Size         string
	Color        string
	UnitsSold    int64
	RevenueMinor int64
}

// SizeRow is one row of the size curve.
type SizeRow struct {
	Size      string
	UnitsSold int64
}

// ChannelRevenue is one row of revenue-by-channel.
type ChannelRevenue struct {
	Channel      string
	OrderCount   int64
	RevenueMinor int64
}

// Valuation is the inventory valuation snapshot.
type Valuation struct {
	InventoryCostMinor   int64
	PotentialRetailMinor int64
	NonSellableCostMinor int64
}

// PL is a profit-and-loss period (ADR-0007).
type PL struct {
	RevenueMinor int64
	COGSMinor    int64
	GrossMinor   int64
	OpExMinor    int64
	NetMinor     int64
	OrderCount   int64
}
