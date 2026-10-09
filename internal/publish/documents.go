package publish

import (
	"encoding/json"
	"time"

	"github.com/akshualy/api.merframe.net/internal/market"
	"github.com/akshualy/api.merframe.net/internal/riven"
)

const goodRollsAttribution = "Good rolls by 44bananas and Xennethkeisere"

type itemPrice struct {
	SellR0        *int        `json:"sell_r0"`
	SellMax       *int        `json:"sell_max"`
	SellR0Online  *int        `json:"sell_r0_online"`
	SellMaxOnline *int        `json:"sell_max_online"`
	BuyR0         *int        `json:"buy_r0"`
	BuyMax        *int        `json:"buy_max"`
	SellCount     int         `json:"sell_count"`
	BuyCount      int         `json:"buy_count"`
	Volume48h     int         `json:"volume_48h"`
	Median48h     *float64    `json:"median_48h"`
	Volume7d      int         `json:"volume_7d"`
	Volume30d     int         `json:"volume_30d"`
	Volume90d     int         `json:"volume_90d"`
	Median7d      *float64    `json:"median_7d"`
	Median30d     *float64    `json:"median_30d"`
	Median90d     *float64    `json:"median_90d"`
	Min90d        *float64    `json:"min_90d"`
	Max90d        *float64    `json:"max_90d"`
	DaysTraded90d int         `json:"days_traded_90d"`
	VolumePrev48h int         `json:"volume_prev_48h"`
	MedianPrev48h *float64    `json:"median_prev_48h"`
	VolumePrev7d  int         `json:"volume_prev_7d"`
	MedianPrev7d  *float64    `json:"median_prev_7d"`
	VolumePrev30d int         `json:"volume_prev_30d"`
	MedianPrev30d *float64    `json:"median_prev_30d"`
	Tier          market.Tier `json:"tier"`
	UpdatedAt     int64       `json:"updated_at"`
	StatisticsAt  int64       `json:"statistics_at"`
}

type priceTable struct {
	UpdatedAt int64                `json:"updated_at"`
	Count     int                  `json:"count"`
	Items     map[string]itemPrice `json:"items"`
}

func encodePrices(prices map[string]market.Price) ([]byte, error) {
	table := priceTable{Count: len(prices), Items: make(map[string]itemPrice, len(prices))}
	for slug, price := range prices {
		table.Items[slug] = itemPriceOf(price)
		if at := price.UpdatedAt.Unix(); at > table.UpdatedAt {
			table.UpdatedAt = at
		}
	}
	return json.Marshal(table)
}

func itemPriceOf(p market.Price) itemPrice {
	return itemPrice{
		SellR0:        p.SellR0,
		SellMax:       p.SellMax,
		SellR0Online:  p.SellR0Online,
		SellMaxOnline: p.SellMaxOnline,
		BuyR0:         p.BuyR0,
		BuyMax:        p.BuyMax,
		SellCount:     p.SellCount,
		BuyCount:      p.BuyCount,
		Volume48h:     p.Volume48h,
		Median48h:     p.Median48h,
		Volume7d:      p.Volume7d,
		Volume30d:     p.Volume30d,
		Volume90d:     p.Volume90d,
		Median7d:      p.Median7d,
		Median30d:     p.Median30d,
		Median90d:     p.Median90d,
		Min90d:        p.Min90d,
		Max90d:        p.Max90d,
		DaysTraded90d: p.DaysTraded90d,
		VolumePrev48h: p.VolumePrev48h,
		MedianPrev48h: p.MedianPrev48h,
		VolumePrev7d:  p.VolumePrev7d,
		MedianPrev7d:  p.MedianPrev7d,
		VolumePrev30d: p.VolumePrev30d,
		MedianPrev30d: p.MedianPrev30d,
		Tier:          p.Tier,
		UpdatedAt:     p.UpdatedAt.Unix(),
		StatisticsAt:  p.StatisticsAt.Unix(),
	}
}

type dailyPrice struct {
	Day    string  `json:"day"`
	Volume int     `json:"volume"`
	Median float64 `json:"median"`
	Min    float64 `json:"min"`
	Max    float64 `json:"max"`
	WA     float64 `json:"wa"`
}

type priceHistory struct {
	Slug      string       `json:"slug"`
	UpdatedAt int64        `json:"updated_at"`
	Days      []dailyPrice `json:"days"`
}

func PriceHistory(slug string, p market.Price, days []market.Day) ([]byte, error) {
	history := priceHistory{Slug: slug, UpdatedAt: p.UpdatedAt.Unix(), Days: make([]dailyPrice, len(days))}
	for i, day := range days {
		history.Days[i] = dailyPrice{
			Day: day.Day.Format("2006-01-02"), Volume: day.Volume,
			Median: day.Median, Min: day.Min, Max: day.Max, WA: day.WA,
		}
	}
	return json.Marshal(history)
}

type rivenDocument struct {
	UpdatedAt          int64                     `json:"updated_at"`
	WeaponsUpdatedAt   *int64                    `json:"weapons_updated_at"`
	GoodRollsUpdatedAt *int64                    `json:"good_rolls_updated_at"`
	Attribution        string                    `json:"attribution"`
	Weapons            []riven.Weapon            `json:"weapons"`
	Attributes         []riven.Attribute         `json:"attributes"`
	GoodRolls          map[string]riven.GoodRoll `json:"good_rolls"`
}

func encodeRiven(d riven.Document) ([]byte, error) {
	doc := rivenDocument{
		UpdatedAt:          max(unixOrZero(d.WeaponsUpdatedAt), unixOrZero(d.GoodRollsUpdatedAt)),
		WeaponsUpdatedAt:   unixOrNil(d.WeaponsUpdatedAt),
		GoodRollsUpdatedAt: unixOrNil(d.GoodRollsUpdatedAt),
		Attribution:        goodRollsAttribution,
		Weapons:            orEmpty(d.Weapons),
		Attributes:         orEmpty(d.Attributes),
		GoodRolls:          d.GoodRolls,
	}
	if doc.GoodRolls == nil {
		doc.GoodRolls = map[string]riven.GoodRoll{}
	}
	return json.Marshal(doc)
}

type weaponAuctions struct {
	Slug      string          `json:"slug"`
	UpdatedAt int64           `json:"updated_at"`
	Truncated bool            `json:"truncated"`
	Auctions  []riven.Auction `json:"auctions"`
}

func encodeAuctions(w riven.WeaponAuctions) ([]byte, error) {
	return json.Marshal(weaponAuctions{
		Slug: w.Slug, UpdatedAt: w.UpdatedAt.Unix(), Truncated: w.Truncated, Auctions: orEmpty(w.Auctions),
	})
}

func encodeCatalog(c market.Catalog) ([]byte, error) {
	return json.Marshal(struct {
		Data json.RawMessage `json:"data"`
	}{Data: c.Document})
}

func unixOrZero(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.Unix()
}

func unixOrNil(t time.Time) *int64 {
	if t.IsZero() {
		return nil
	}
	return new(t.Unix())
}

func orEmpty[T any](list []T) []T {
	if list == nil {
		return []T{}
	}
	return list
}
