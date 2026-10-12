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
	v := orderVariant(item, orders)
	q := Quote{
		SellR0:       lowestSell(ingame, v.order),
		SellR0Online: lowestSell(active, v.order),
		BuyR0:        highestBuy(active, v.order),
	}
	if item.MaxRank != nil && *item.MaxRank > 0 {
		q.SellMax = lowestSell(ingame, v.topRank)
		q.SellMaxOnline = lowestSell(active, v.topRank)
		q.BuyMax = highestBuy(active, v.topRank)
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
