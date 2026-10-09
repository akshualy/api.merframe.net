package market

import (
	"time"

	"github.com/akshualy/api.merframe.net/internal/wfm"
)

type Tier string

const (
	TierPrime Tier = "prime"
	TierHot   Tier = "hot"
	TierCold  Tier = "cold"
)

type Price struct {
	Quote
	Statistics
	Tier         Tier
	UpdatedAt    time.Time
	StatisticsAt time.Time
}

type Catalog struct {
	wfm.Catalog
	RefreshedAt time.Time
}

func tierOf(prime, active bool) Tier {
	switch {
	case prime:
		return TierPrime
	case active:
		return TierHot
	default:
		return TierCold
	}
}
