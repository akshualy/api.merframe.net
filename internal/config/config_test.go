package config

import (
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

func env(pairs map[string]string) func(string) string {
	return func(name string) string { return pairs[name] }
}

var defaults = Collector{
	DatabaseURL:         "postgres://x",
	Contact:             "https://merframe.net",
	MarketRate:          1,
	PrimeInterval:       30 * time.Minute,
	HotInterval:         4 * time.Hour,
	ColdInterval:        24 * time.Hour,
	StatisticsInterval:  24 * time.Hour,
	CatalogInterval:     24 * time.Hour,
	PublishInterval:     15 * time.Minute,
	RivenInterval:       24 * time.Hour,
	AuctionPassInterval: 24 * time.Hour,
	AuctionSpacing:      6 * time.Second,
}

func TestLoadCollector(t *testing.T) {
	overridden := defaults
	overridden.MarketRate = 0.5
	overridden.HotInterval = 2 * time.Hour
	tests := []struct {
		name    string
		env     map[string]string
		want    Collector
		wantErr string
	}{
		{
			name: "defaults",
			env:  map[string]string{"MERFRAME_DATABASE_URL": "postgres://x"},
			want: defaults,
		},
		{
			name: "overrides",
			env: map[string]string{
				"MERFRAME_DATABASE_URL": "postgres://x",
				"MERFRAME_MARKET_RATE":  "0.5",
				"MERFRAME_HOT_INTERVAL": "2h",
			},
			want: overridden,
		},
		{
			name:    "database url required",
			env:     map[string]string{},
			wantErr: `MERFRAME_DATABASE_URL is "", want a value`,
		},
		{
			name:    "bad duration",
			env:     map[string]string{"MERFRAME_DATABASE_URL": "postgres://x", "MERFRAME_COLD_INTERVAL": "soon"},
			wantErr: `MERFRAME_COLD_INTERVAL is "soon"`,
		},
		{
			name:    "negative duration",
			env:     map[string]string{"MERFRAME_DATABASE_URL": "postgres://x", "MERFRAME_PUBLISH_INTERVAL": "-5m"},
			wantErr: `MERFRAME_PUBLISH_INTERVAL is "-5m"`,
		},
		{
			name:    "zero rate",
			env:     map[string]string{"MERFRAME_DATABASE_URL": "postgres://x", "MERFRAME_MARKET_RATE": "0"},
			wantErr: `MERFRAME_MARKET_RATE is "0"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := LoadCollector(env(tt.env))
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want %q", err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatal(err)
			}

			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("config differs (-want +got):\n%s", diff)
			}
		})
	}
}

func TestLoadAPI(t *testing.T) {
	tests := []struct {
		name    string
		env     map[string]string
		want    API
		wantErr string
	}{
		{
			name: "defaults",
			env:  map[string]string{"MERFRAME_DATABASE_URL": "postgres://x"},
			want: API{DatabaseURL: "postgres://x", ListenAddr: "127.0.0.1:8080"},
		},
		{
			name: "listen address",
			env:  map[string]string{"MERFRAME_DATABASE_URL": "postgres://x", "MERFRAME_LISTEN_ADDR": ":8080"},
			want: API{DatabaseURL: "postgres://x", ListenAddr: ":8080"},
		},
		{
			name:    "database url required",
			env:     map[string]string{},
			wantErr: "MERFRAME_DATABASE_URL",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := LoadAPI(env(tt.env))
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want %q", err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatal(err)
			}

			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("config differs (-want +got):\n%s", diff)
			}
		})
	}
}
