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
func quoteOf(orders []wfm.Order, maxRank *int) Quote {
	ingame := tradeable(orders, "ingame")
	active := tradeable(orders, "ingame", "online")
	q := Quote{
		SellR0:       lowestSell(ingame, baseRank),
		SellR0Online: lowestSell(active, baseRank),
		BuyR0:        highestBuy(active, baseRank),
	}
	if maxRank != nil && *maxRank > 0 {
		top := func(rank *int) bool { return rank != nil && *rank == *maxRank }
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

func lowestSell(orders []wfm.Order, atRank func(*int) bool) *int {
	var best *int
	for _, order := range orders {
		if order.Type == "sell" && atRank(order.Rank) && (best == nil || order.Platinum < *best) {
			best = new(order.Platinum)
		}
	}
	return best
}

func highestBuy(orders []wfm.Order, atRank func(*int) bool) *int {
	var best *int
	for _, order := range orders {
		if order.Type == "buy" && atRank(order.Rank) && (best == nil || order.Platinum > *best) {
			best = new(order.Platinum)
		}
	}
	return best
}
