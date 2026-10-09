package sheets

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

const (
	goodRollsSheet = "1zbaeJBuBn44cbVKzJins_E3hTDpnmvOk8heYN-G8yy8"
	exportURL      = "https://docs.google.com/spreadsheets/d/" + goodRollsSheet + "/gviz/tq?tqx=out:csv&sheet="
)

type Client struct {
	http      *http.Client
	userAgent string
	base      string
}

func New(hc *http.Client, loop, contact string) *Client {
	return &Client{http: hc, userAgent: fmt.Sprintf("api.merframe.net %s (+%s)", loop, contact), base: exportURL}
}

func (c *Client) Tab(ctx context.Context, name string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+url.QueryEscape(name), nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("User-Agent", c.userAgent)
	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("sheet tab %s answered %d", name, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading sheet tab %s: %w", name, err)
	}
	return string(body), nil
}
