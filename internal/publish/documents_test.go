package publish

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/akshualy/api.merframe.net/internal/market"
	"github.com/akshualy/api.merframe.net/internal/riven"
	"github.com/akshualy/api.merframe.net/internal/wfm"
)

var at = time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)

func decode(t *testing.T) func([]byte, error) map[string]any {
	t.Helper()
	return func(data []byte, err error) map[string]any {
		if err != nil {
			t.Fatal(err)
		}

		var out map[string]any
		if err := json.Unmarshal(data, &out); err != nil {
			t.Fatalf("%s: %v", data, err)
		}
		return out
	}
}

func TestEncodePrices(t *testing.T) {
	prices := map[string]market.Price{
		"forma_blueprint": {
			Quote:      market.Quote{SellR0: new(10), SellCount: 3},
			Statistics: market.Statistics{Volume48h: 7, Median48h: new(9.5)},
			Tier:       market.TierHot, UpdatedAt: at, StatisticsAt: at.Add(-time.Hour),
		},
		"serration": {Tier: market.TierCold, UpdatedAt: at.Add(-time.Minute), StatisticsAt: at.Add(-time.Minute)},
	}
	got := decode(t)(encodePrices(prices))
	if got["count"] != 2.0 || got["updated_at"] != float64(at.Unix()) {
		t.Errorf("count %v, updated_at %v, want 2 and the newest item", got["count"], got["updated_at"])
	}

	forma := got["items"].(map[string]any)["forma_blueprint"].(map[string]any)
	if len(forma) != 28 || forma["sell_r0"] != 10.0 || forma["sell_max"] != nil || forma["median_48h"] != 9.5 || forma["tier"] != "hot" || forma["statistics_at"] != float64(at.Add(-time.Hour).Unix()) {
		t.Errorf("item = %v", forma)
	}
}

func TestPriceHistory(t *testing.T) {
	days := []market.Day{{Day: time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC), Volume: 23, Median: 30, Min: 20, Max: 40, WA: 29.043}}
	got := decode(t)(PriceHistory("secura_dual_cestra", market.Price{UpdatedAt: at}, days))
	day := got["days"].([]any)[0].(map[string]any)
	if got["slug"] != "secura_dual_cestra" || got["updated_at"] != float64(at.Unix()) || day["day"] != "2026-06-30" || day["wa"] != 29.043 {
		t.Errorf("history = %v", got)
	}
}

func TestRiven(t *testing.T) {
	got := decode(t)(encodeRiven(riven.Document{
		WeaponsUpdatedAt: at,
		Weapons:          []riven.Weapon{{Slug: "torid", Name: "Torid"}},
	}))
	if got["updated_at"] != float64(at.Unix()) || got["weapons_updated_at"] != float64(at.Unix()) || got["good_rolls_updated_at"] != nil {
		t.Errorf("timestamps %v %v %v, want the weapons time twice and null", got["updated_at"], got["weapons_updated_at"], got["good_rolls_updated_at"])
	}

	if got["attribution"] != goodRollsAttribution {
		t.Errorf("attribution = %v", got["attribution"])
	}

	if _, isList := got["attributes"].([]any); !isList {
		t.Errorf("attributes = %v, want an empty array", got["attributes"])
	}

	if _, isMap := got["good_rolls"].(map[string]any); !isMap {
		t.Errorf("good_rolls = %v, want an empty object", got["good_rolls"])
	}
}

func TestEncodeAuctionsAndCatalog(t *testing.T) {
	got := decode(t)(encodeAuctions(riven.WeaponAuctions{Slug: "torid", UpdatedAt: at, Truncated: true}))
	if got["slug"] != "torid" || got["truncated"] != true {
		t.Errorf("auctions = %v", got)
	}

	if _, isList := got["auctions"].([]any); !isList {
		t.Errorf("auctions = %v, want an empty array", got["auctions"])
	}

	catalog := decode(t)(encodeCatalog(market.Catalog{Catalog: wfm.Catalog{Document: []byte(`[{"slug":"forma_blueprint"}]`)}}))
	if data, ok := catalog["data"].([]any); !ok || len(data) != 1 {
		t.Errorf("catalog = %v, want data with one item", catalog)
	}
}
