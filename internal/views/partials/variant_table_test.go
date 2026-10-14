package partials

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/druidswebdesign/varels-cms/internal/db/sqlc"
)

// TestVariantTableShowsStockControls guards the product-page stock affordance:
// each variant row must show an editable sellable on-hand quantity that posts to
// the set-quantity endpoint, plus a link to the receive/adjust editor.
func TestVariantTableShowsStockControls(t *testing.T) {
	variants := []sqlc.ProductVariant{{ID: 7, Sku: "SKU-1"}}
	comp := VariantTable(42, variants, map[int64]int64{7: 12}, "tok")

	var buf bytes.Buffer
	if err := comp.Render(context.Background(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	html := buf.String()

	for _, want := range []string{
		"On hand",
		"/variants/7/quantity",
		`name="quantity"`,
		`value="12"`,
		"/variants/7/restock",
		">Stock</a>",
	} {
		if !strings.Contains(html, want) {
			t.Errorf("variant table missing %q\n%s", want, html)
		}
	}
}
