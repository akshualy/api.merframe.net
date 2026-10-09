package riven

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/akshualy/api.merframe.net/internal/wfm"
)

func readJSON(t *testing.T, dir, name string, v any) {
	t.Helper()

	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", dir, name))
	if err != nil {
		t.Fatal(err)
	}

	if err := json.Unmarshal(data, v); err != nil {
		t.Fatalf("%s: %v", name, err)
	}
}

func TestLists(t *testing.T) {
	var expected struct {
		Weapons    []Weapon    `json:"weapons"`
		Attributes []Attribute `json:"attributes"`
	}
	readJSON(t, "expected", "riven_lists.json", &expected)
	var weaponsFixture struct {
		Data []wfm.RivenWeapon `json:"data"`
	}
	readJSON(t, "wfm", "riven_weapons.json", &weaponsFixture)
	var attributesFixture struct {
		Data []wfm.RivenAttribute `json:"data"`
	}
	readJSON(t, "wfm", "riven_attributes.json", &attributesFixture)

	if diff := cmp.Diff(expected.Weapons, weapons(weaponsFixture.Data)); diff != "" {
		t.Errorf("weapons (-want +got):\n%s", diff)
	}

	if diff := cmp.Diff(expected.Attributes, attributes(attributesFixture.Data)); diff != "" {
		t.Errorf("attributes (-want +got):\n%s", diff)
	}
}

func TestOpenAuctions(t *testing.T) {
	var expected []Auction
	readJSON(t, "expected", "auctions.json", &expected)
	var search struct {
		Payload struct {
			Auctions []wfm.Auction `json:"auctions"`
		} `json:"payload"`
	}
	readJSON(t, "wfm", "auctions_search.json", &search)
	if diff := cmp.Diff(expected, openAuctions(search.Payload.Auctions)); diff != "" {
		t.Errorf("auctions (-want +got):\n%s", diff)
	}
}
