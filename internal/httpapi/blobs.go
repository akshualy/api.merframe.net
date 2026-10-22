package httpapi

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/akshualy/api.merframe.net/internal/blob"
	"github.com/akshualy/api.merframe.net/internal/publish"
)

type Blobs struct {
	mu      sync.RWMutex
	blobs   map[string]*blob.Blob
	items   map[string]json.RawMessage
	weapons map[string]struct{}
}

func NewBlobs() *Blobs {
	return &Blobs{blobs: map[string]*blob.Blob{}, items: map[string]json.RawMessage{}, weapons: map[string]struct{}{}}
}

// Set indexes the price table and the riven weapons. Items served will be the same as the table's.
func (b *Blobs) Set(name string, value *blob.Blob) error {
	b.mu.Lock()
	defer b.mu.Unlock()
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
