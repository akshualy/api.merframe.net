package httpapi

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/akshualy/api.merframe.net/internal/blob"
	"github.com/akshualy/api.merframe.net/internal/publish"
)

type Blobs struct {
	mu    sync.RWMutex
	blobs map[string]*blob.Blob
	items map[string]json.RawMessage
}

func NewBlobs() *Blobs {
	return &Blobs{blobs: map[string]*blob.Blob{}, items: map[string]json.RawMessage{}}
}

// Set indexes the items of the price table. Items served will be the same as the table's.
func (b *Blobs) Set(name string, value *blob.Blob) error {
	var items map[string]json.RawMessage
	if name == publish.PricesBlob {
		var table struct {
			Items map[string]json.RawMessage `json:"items"`
		}
		if err := json.Unmarshal(value.Body, &table); err != nil {
			return fmt.Errorf("decoding the price table: %w", err)
		}

		items = table.Items
	}

	b.mu.Lock()
	defer b.mu.Unlock()
	b.blobs[name] = value

	if items != nil {
		b.items = items
	}
	return nil
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
