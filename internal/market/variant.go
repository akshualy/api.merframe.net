package market

import "github.com/akshualy/api.merframe.net/internal/wfm"

// variant is the base rank, the first catalog subtype, and for sculptures the fully socketed one.
type variant struct {
	item wfm.Item
	full sockets
}

type sockets struct {
	cyan, amber int
}

func orderVariant(item wfm.Item, orders []wfm.Order) variant {
	v := variant{item: item}
	for _, o := range orders {
		v.full = v.full.max(sockets{o.CyanStars, o.AmberStars})
	}
	return v
}

func closedVariant(item wfm.Item, closed wfm.ClosedStatistics) variant {
	v := variant{item: item}
	for _, e := range closed.Last90Days {
		v.full = v.full.max(sockets{e.CyanStars, e.AmberStars})
	}

	for _, e := range closed.Last48Hours {
		v.full = v.full.max(sockets{e.CyanStars, e.AmberStars})
	}
	return v
}

func (v variant) order(o wfm.Order) bool {
	return baseRank(o.Rank) && v.subtype(o.Subtype) && v.full == sockets{o.CyanStars, o.AmberStars}
}

func (v variant) topRank(o wfm.Order) bool {
	return o.Rank != nil && *o.Rank == *v.item.MaxRank && v.subtype(o.Subtype) && v.full == sockets{o.CyanStars, o.AmberStars}
}

func (v variant) entry(e wfm.ClosedEntry) bool {
	return baseRank(e.ModRank) && v.subtype(e.Subtype) && v.full == sockets{e.CyanStars, e.AmberStars}
}

func (v variant) subtype(subtype *string) bool {
	return subtype == nil || len(v.item.Subtypes) == 0 || *subtype == v.item.Subtypes[0]
}

func baseRank(rank *int) bool {
	return rank == nil || *rank == 0
}

func (s sockets) max(o sockets) sockets {
	return sockets{cyan: max(s.cyan, o.cyan), amber: max(s.amber, o.amber)}
}
