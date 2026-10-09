package market

import (
	"testing"
)

func TestTierOf(t *testing.T) {
	tests := []struct {
		prime, active bool
		want          Tier
	}{
		{true, false, TierPrime},
		{true, true, TierPrime},
		{false, true, TierHot},
		{false, false, TierCold},
	}
	for _, tt := range tests {
		if got := tierOf(tt.prime, tt.active); got != tt.want {
			t.Errorf("tierOf(%v, %v) = %s, want %s", tt.prime, tt.active, got, tt.want)
		}
	}
}
