package market

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/akshualy/api.merframe.net/internal/wfm"
)

func readJSON(t *testing.T, dir, name string, v any) {
	t.Helper()

	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", dir, name))
	if err != nil {
		t.Fatal(err)
	}

	if err := json.Unmarshal(data, v); err != nil {
		t.Fatalf("%s: %v", name, err)
	}
}

type expectedQuote struct {
	SellR0        *int `json:"sell_r0"`
	SellMax       *int `json:"sell_max"`
	SellR0Online  *int `json:"sell_r0_online"`
	SellMaxOnline *int `json:"sell_max_online"`
	BuyR0         *int `json:"buy_r0"`
	BuyMax        *int `json:"buy_max"`
	SellCount     int  `json:"sell_count"`
	BuyCount      int  `json:"buy_count"`
}

var relic = wfm.Item{Subtypes: []string{"intact", "exceptional", "flawless", "radiant"}}

func TestQuote(t *testing.T) {
	var expected map[string]expectedQuote
	readJSON(t, "expected", "orders.json", &expected)
	tests := []struct {
		slug    string
		fixture string
		item    wfm.Item
	}{
		{"secura_dual_cestra", "orders_item.json", wfm.Item{}},
		{"primed_flow", "orders_item_ranked.json", wfm.Item{MaxRank: new(10)}},
		{"neo_k6_relic", "orders_relic.json", relic},
		{"ayatan_anasa_sculpture", "orders_sculpture.json", wfm.Item{}},
	}
	for _, tt := range tests {
		t.Run(tt.slug, func(t *testing.T) {
			var envelope struct {
				Data []wfm.Order `json:"data"`
			}
			readJSON(t, "wfm", tt.fixture, &envelope)
			got := quoteOf(envelope.Data, tt.item)
			if diff := cmp.Diff(Quote(expected[tt.slug]), got); diff != "" {
				t.Errorf("quote (-want +got):\n%s", diff)
			}
		})
	}
}
