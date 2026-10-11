package market

import (
	"slices"

	"github.com/akshualy/api.merframe.net/internal/wfm"
)

type Quote struct {
	SellR0        *int
	SellMax       *int
	SellR0Online  *int
	SellMaxOnline *int
	BuyR0         *int
	BuyMax        *int
	SellCount     int
	BuyCount      int
}

// quoteOf takes sell_r0 and sell_max from in-game sellers only, the online variants and the buys from in-game or online players.
func quoteOf(orders []wfm.Order, item wfm.Item) Quote {
	ingame := tradeable(orders, "ingame")
	active := tradeable(orders, "ingame", "online")
	base := func(o wfm.Order) bool { return baseRank(o.Rank) && baseSubtype(item, o.Subtype) }
	q := Quote{
		SellR0:       lowestSell(ingame, base),
		SellR0Online: lowestSell(active, base),
		BuyR0:        highestBuy(active, base),
	}
	if item.MaxRank != nil && *item.MaxRank > 0 {
		top := func(o wfm.Order) bool {
			return o.Rank != nil && *o.Rank == *item.MaxRank && baseSubtype(item, o.Subtype)
		}
		q.SellMax = lowestSell(ingame, top)
		q.SellMaxOnline = lowestSell(active, top)
		q.BuyMax = highestBuy(active, top)
	}

	for _, order := range active {
		switch order.Type {
		case "sell":
			q.SellCount++
		case "buy":
			q.BuyCount++
		}
	}
	return q
}

func (q Quote) active() bool {
	return q.SellCount > 0 || q.BuyCount > 0
}

func tradeable(orders []wfm.Order, statuses ...string) []wfm.Order {
	var out []wfm.Order
	for _, order := range orders {
		if order.Visible && slices.Contains(statuses, order.User.Status) {
			out = append(out, order)
		}
	}
	return out
}

func baseRank(rank *int) bool {
	return rank == nil || *rank == 0
}

// baseSubtype accepts the first subtype the catalog lists for the item (intact for relics, small for fish) and entries without one.
func baseSubtype(item wfm.Item, subtype *string) bool {
	return subtype == nil || len(item.Subtypes) == 0 || *subtype == item.Subtypes[0]
}

func lowestSell(orders []wfm.Order, variant func(wfm.Order) bool) *int {
	var best *int
	for _, order := range orders {
		if order.Type == "sell" && variant(order) && (best == nil || order.Platinum < *best) {
			best = new(order.Platinum)
		}
	}
	return best
}

func highestBuy(orders []wfm.Order, variant func(wfm.Order) bool) *int {
	var best *int
	for _, order := range orders {
		if order.Type == "buy" && variant(order) && (best == nil || order.Platinum > *best) {
			best = new(order.Platinum)
		}
	}
	return best
}
