package httpapi

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/akshualy/api.merframe.net/internal/blob"
	"github.com/akshualy/api.merframe.net/internal/publish"
)

type Blobs struct {
	mu      sync.RWMutex
	blobs   map[string]*blob.Blob
	items   map[string]json.RawMessage
	weapons map[string]struct{}
	crawled map[string]time.Time
}

func NewBlobs() *Blobs {
	return &Blobs{
		blobs:   map[string]*blob.Blob{},
		items:   map[string]json.RawMessage{},
		weapons: map[string]struct{}{},
		crawled: map[string]time.Time{},
	}
}

// Set indexes the price table, the riven weapons, and the crawl timestamp of auction documents. Items served will be the same as the table's.
func (b *Blobs) Set(name string, value *blob.Blob) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if slug, ok := publish.AuctionsSlug(name); ok {
		crawledAt, err := decodeCrawledAt(value.Body)
		if err != nil {
			return err
		}

		b.crawled[slug] = crawledAt
	}

	switch name {
	case publish.PricesBlob:
		items, err := decodeItems(value.Body)
		if err != nil {
			return err
		}

		b.items = items
	case publish.RivenBlob:
		weapons, err := decodeWeapons(value.Body)
		if err != nil {
			return err
		}

		b.weapons = weapons
	}

	b.blobs[name] = value
	return nil
}

func decodeItems(body []byte) (map[string]json.RawMessage, error) {
	var table struct {
		Items map[string]json.RawMessage `json:"items"`
	}
	if err := json.Unmarshal(body, &table); err != nil {
		return nil, fmt.Errorf("decoding the price table: %w", err)
	}
	return table.Items, nil
}

func decodeWeapons(body []byte) (map[string]struct{}, error) {
	var document struct {
		Weapons []struct {
			Slug string `json:"slug"`
		} `json:"weapons"`
	}
	if err := json.Unmarshal(body, &document); err != nil {
		return nil, fmt.Errorf("decoding the riven document: %w", err)
	}

	weapons := make(map[string]struct{}, len(document.Weapons))
	for _, weapon := range document.Weapons {
		weapons[weapon.Slug] = struct{}{}
	}
	return weapons, nil
}

func decodeCrawledAt(body []byte) (time.Time, error) {
	var document struct {
		UpdatedAt int64 `json:"updated_at"`
	}
	if err := json.Unmarshal(body, &document); err != nil {
		return time.Time{}, fmt.Errorf("decoding the auction document: %w", err)
	}
	return time.Unix(document.UpdatedAt, 0), nil
}

func (b *Blobs) Get(name string) (*blob.Blob, bool) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	value, ok := b.blobs[name]
	return value, ok
}

func (b *Blobs) Item(slug string) (json.RawMessage, bool) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	item, ok := b.items[slug]
	return item, ok
}

func (b *Blobs) Auctions(slug string) (*blob.Blob, time.Time, bool) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	value, ok := b.blobs[publish.AuctionsBlob(slug)]
	return value, b.crawled[slug], ok
}

func (b *Blobs) HasWeapon(slug string) bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	_, ok := b.weapons[slug]
	return ok
}

func (b *Blobs) Ready() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()

	for _, name := range []string{publish.PricesBlob, publish.RivenBlob, publish.CatalogBlob} {
		if _, ok := b.blobs[name]; !ok {
			return false
		}
	}
	return true
}
