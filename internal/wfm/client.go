package wfm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/akshualy/api.merframe.net/internal/throttle"
)

const (
	defaultV1 = "https://api.warframe.market/v1"
	defaultV2 = "https://api.warframe.market/v2"

	maxAttempts        = 5
	firstRetryDelay    = 2 * time.Second
	rateLimitedDefault = 30 * time.Second
	retryAfterSlack    = time.Second
)

var ErrNotFound = errors.New("not found on warframe.market")

type StatusError struct {
	Status int
	URL    string
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("%s answered %d", e.URL, e.Status)
}

type Client struct {
	http      *http.Client
	throttle  *throttle.Throttle
	userAgent string
	v1        string
	v2        string
}

func New(hc *http.Client, t *throttle.Throttle, loop, contact string) *Client {
	return &Client{
		http:      hc,
		throttle:  t,
		userAgent: fmt.Sprintf("api.merframe.net %s (+%s)", loop, contact),
		v1:        defaultV1,
		v2:        defaultV2,
	}
}

func (c *Client) Items(ctx context.Context) (Catalog, error) {
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := c.get(ctx, c.v2+"/items", &envelope); err != nil {
		return Catalog{}, err
	}

	var items []Item
	if err := json.Unmarshal(envelope.Data, &items); err != nil {
		return Catalog{}, fmt.Errorf("decoding the item list: %w", err)
	}
	return Catalog{Items: items, Document: envelope.Data}, nil
}

func (c *Client) Orders(ctx context.Context, slug string) ([]Order, error) {
	var envelope struct {
		Data []Order `json:"data"`
	}
	if err := c.get(ctx, c.v2+"/orders/item/"+url.PathEscape(slug), &envelope); err != nil {
		return nil, err
	}
	return envelope.Data, nil
}

func (c *Client) Statistics(ctx context.Context, slug string) (ClosedStatistics, error) {
	var envelope struct {
		Payload struct {
			Closed ClosedStatistics `json:"statistics_closed"`
		} `json:"payload"`
	}
	if err := c.get(ctx, c.v1+"/items/"+url.PathEscape(slug)+"/statistics", &envelope); err != nil {
		return ClosedStatistics{}, err
	}
	return envelope.Payload.Closed, nil
}

func (c *Client) RivenWeapons(ctx context.Context) ([]RivenWeapon, error) {
	var envelope struct {
		Data []RivenWeapon `json:"data"`
	}
	if err := c.get(ctx, c.v2+"/riven/weapons", &envelope); err != nil {
		return nil, err
	}
	return envelope.Data, nil
}

func (c *Client) RivenAttributes(ctx context.Context) ([]RivenAttribute, error) {
	var envelope struct {
		Data []RivenAttribute `json:"data"`
	}
	if err := c.get(ctx, c.v2+"/riven/attributes", &envelope); err != nil {
		return nil, err
	}
	return envelope.Data, nil
}

// SearchAuctions returns up to 500 listings, cheapest first.
func (c *Client) SearchAuctions(ctx context.Context, search AuctionSearch) ([]Auction, error) {
	query := url.Values{
		"type":            {"riven"},
		"weapon_url_name": {search.Weapon},
		"sort_by":         {"price_asc"},
	}
	if search.Polarity != "" {
		query.Set("polarity", search.Polarity)
	}

	if search.ReRollsMin != nil {
		query.Set("re_rolls_min", strconv.Itoa(*search.ReRollsMin))
	}

	if search.ReRollsMax != nil {
		query.Set("re_rolls_max", strconv.Itoa(*search.ReRollsMax))
	}

	var envelope struct {
		Payload struct {
			Auctions []Auction `json:"auctions"`
		} `json:"payload"`
	}
	if err := c.get(ctx, c.v1+"/auctions/search?"+query.Encode(), &envelope); err != nil {
		return nil, err
	}
	return envelope.Payload.Auctions, nil
}

func (c *Client) get(ctx context.Context, target string, dst any) error {
	delay := firstRetryDelay
	for attempt := 1; ; attempt++ {
		if err := c.throttle.Wait(ctx); err != nil {
			return err
		}

		status, retryAfter, err := c.fetch(ctx, target, dst)
		if err != nil {
			return err
		}

		switch {
		case status == http.StatusOK:
			return nil
		case status == http.StatusNotFound:
			return ErrNotFound
		case attempt == maxAttempts:
			return &StatusError{Status: status, URL: target}
		case status == http.StatusTooManyRequests:
			c.throttle.Hold(retryAfter)
		case retryable(status):
			c.throttle.Hold(delay)
			delay *= 2
		default:
			return &StatusError{Status: status, URL: target}
		}
	}
}

func (c *Client) fetch(ctx context.Context, target string, dst any) (int, time.Duration, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return 0, 0, err
	}

	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Platform", "pc")
	req.Header.Set("Language", "en")
	resp, err := c.http.Do(req)
	if err != nil {
		return 0, 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, resp.Body)
		return resp.StatusCode, retryAfter(resp.Header), nil
	}

	if err := json.NewDecoder(resp.Body).Decode(dst); err != nil {
		return 0, 0, fmt.Errorf("decoding %s: %w", target, err)
	}
	return resp.StatusCode, 0, nil
}

func retryable(status int) bool {
	switch status {
	case http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	}
	return false
}

func retryAfter(h http.Header) time.Duration {
	seconds, err := strconv.Atoi(h.Get("Retry-After"))
	if err != nil {
		return rateLimitedDefault
	}
	return time.Duration(seconds)*time.Second + retryAfterSlack
}
