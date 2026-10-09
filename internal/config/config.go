package config

import (
	"fmt"
	"strconv"
	"time"
)

const prefix = "MERFRAME_"

type Collector struct {
	DatabaseURL         string
	Contact             string
	MarketRate          float64
	PrimeInterval       time.Duration
	HotInterval         time.Duration
	ColdInterval        time.Duration
	StatisticsInterval  time.Duration
	CatalogInterval     time.Duration
	PublishInterval     time.Duration
	RivenInterval       time.Duration
	AuctionPassInterval time.Duration
	AuctionSpacing      time.Duration
}

type API struct {
	DatabaseURL string
	ListenAddr  string
}

func LoadCollector(getenv func(string) string) (Collector, error) {
	env := reader{getenv: getenv}
	c := Collector{
		DatabaseURL:         env.required("DATABASE_URL"),
		Contact:             env.text("CONTACT", "https://merframe.net"),
		MarketRate:          env.rate("MARKET_RATE", 1.0),
		PrimeInterval:       env.duration("PRIME_INTERVAL", 30*time.Minute),
		HotInterval:         env.duration("HOT_INTERVAL", 4*time.Hour),
		ColdInterval:        env.duration("COLD_INTERVAL", 24*time.Hour),
		StatisticsInterval:  env.duration("STATISTICS_INTERVAL", 24*time.Hour),
		CatalogInterval:     env.duration("CATALOG_INTERVAL", 24*time.Hour),
		PublishInterval:     env.duration("PUBLISH_INTERVAL", 15*time.Minute),
		RivenInterval:       env.duration("RIVEN_INTERVAL", 24*time.Hour),
		AuctionPassInterval: env.duration("AUCTION_PASS_INTERVAL", 24*time.Hour),
		AuctionSpacing:      env.duration("AUCTION_SPACING", 6*time.Second),
	}
	if env.err != nil {
		return Collector{}, env.err
	}
	return c, nil
}

func LoadAPI(getenv func(string) string) (API, error) {
	env := reader{getenv: getenv}
	a := API{
		DatabaseURL: env.required("DATABASE_URL"),
		ListenAddr:  env.text("LISTEN_ADDR", "127.0.0.1:8080"),
	}
	if env.err != nil {
		return API{}, env.err
	}
	return a, nil
}

type reader struct {
	getenv func(string) string
	err    error
}

func (r *reader) required(name string) string {
	value := r.getenv(prefix + name)
	if value == "" {
		r.fail(name, value, "a value")
	}
	return value
}

func (r *reader) text(name, fallback string) string {
	if value := r.getenv(prefix + name); value != "" {
		return value
	}
	return fallback
}

func (r *reader) duration(name string, fallback time.Duration) time.Duration {
	value := r.getenv(prefix + name)
	if value == "" {
		return fallback
	}

	d, err := time.ParseDuration(value)
	if err != nil || d <= 0 {
		r.fail(name, value, "a positive duration such as 30m")
		return fallback
	}
	return d
}

func (r *reader) rate(name string, fallback float64) float64 {
	value := r.getenv(prefix + name)
	if value == "" {
		return fallback
	}

	f, err := strconv.ParseFloat(value, 64)
	if err != nil || f <= 0 {
		r.fail(name, value, "a positive number of requests per second")
		return fallback
	}
	return f
}

func (r *reader) fail(name, value, want string) {
	if r.err == nil {
		r.err = fmt.Errorf("%s%s is %q, want %s", prefix, name, value, want)
	}
}
