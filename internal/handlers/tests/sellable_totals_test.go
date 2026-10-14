package tests

import (
	"context"
	"testing"
)

// TestListSellableTotalsByProduct verifies the on-hand aggregate used by the
// product page's variant table: it must sum sellable stock per variant.
func TestListSellableTotalsByProduct(t *testing.T) {
	e := newTestEnv(t)
	productID := e.scalarInt("SELECT product_id FROM product_variants WHERE sku = 'TEE-BLK-M'")

	rows, err := e.srv.Q.ListSellableTotalsByProduct(context.Background(), productID)
	if err != nil {
		t.Fatalf("ListSellableTotalsByProduct: %v", err)
	}

	got := map[int64]int64{}
	for _, r := range rows {
		got[r.VariantID] = r.OnHand
	}

	// Dev seed: TEE-BLK-M has 8 sellable units, TEE-BLK-L has 5.
	if got[e.variantM] != 8 {
		t.Errorf("on-hand for variant M = %d, want 8", got[e.variantM])
	}
	if got[e.variantL] != 5 {
		t.Errorf("on-hand for variant L = %d, want 5", got[e.variantL])
	}
}
