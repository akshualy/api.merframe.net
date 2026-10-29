package riven

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestSheet(t *testing.T) {
	tests := []struct {
		class, sheet, expected string
	}{
		{"primary", "good_rolls_primary.csv", "good_rolls_primary.json"},
		{"melee", "good_rolls_melee.csv", "good_rolls_melee.json"},
	}
	for _, tt := range tests {
		t.Run(tt.class, func(t *testing.T) {
			sheet, err := os.ReadFile(filepath.Join("..", "..", "testdata", "wfm", tt.sheet))
			if err != nil {
				t.Fatal(err)
			}

			expectedData, err := os.ReadFile(filepath.Join("..", "..", "testdata", "expected", tt.expected))
			if err != nil {
				t.Fatal(err)
			}

			var expected struct {
				Rolls    map[string]GoodRoll `json:"rolls"`
				Unmapped []string            `json:"unmapped"`
			}
			if err := json.Unmarshal(expectedData, &expected); err != nil {
				t.Fatal(err)
			}

			parser := newSheetParser(tt.class)
			rolls, err := parser.parse(string(sheet))
			if err != nil {
				t.Fatal(err)
			}

			if diff := cmp.Diff(expected.Rolls, rolls); diff != "" {
				t.Errorf("rolls (-want +got):\n%s", diff)
			}

			if diff := cmp.Diff(expected.Unmapped, parser.unmapped()); diff != "" {
				t.Errorf("unmapped differ (-want +got):\n%s", diff)
			}
		})
	}
}

func TestCollapse(t *testing.T) {
	sheet := "\"WEAPON\",\"POSITIVE STATS:\",\"NEGATIVE STATS:\"\n\"torid\",\"TOX DTC or TOX DTG or TOX DTI\",\"IMP\"\n"
	rolls, err := newSheetParser("primary").parse(sheet)
	if err != nil {
		t.Fatal(err)
	}

	alternatives := rolls["torid"].Alternatives
	if len(alternatives) != 1 {
		t.Fatalf("got %d alternatives, want 1", len(alternatives))
	}

	got := alternatives[0]
	if len(got.Mandatory) != 1 || got.Mandatory[0].Abbr != "TOX" || len(got.Optional) != 3 || got.OptionalNeeded != 1 {
		t.Errorf("got %+v, want TOX plus one of three", got)
	}
}

func TestUnmapped(t *testing.T) {
	sheet := "\"WEAPON\",\"POSITIVE STATS:\",\"NEGATIVE STATS:\"\n\"torid\",\"CD WAT\",\"IMP\"\n\"boar\",\"CD CC\",\"WAT\"\n"
	parser := newSheetParser("primary")
	rolls, err := parser.parse(sheet)
	if err != nil {
		t.Fatal(err)
	}

	if _, kept := rolls["torid"]; kept {
		t.Error("got torid, want it dropped for its unknown positive")
	}

	if boar := rolls["boar"]; len(boar.AcceptedBad) != 0 {
		t.Errorf("got AcceptedBad %+v, want none for an unknown negative", boar.AcceptedBad)
	}

	if diff := cmp.Diff([]string{"primary:WAT"}, parser.unmapped()); diff != "" {
		t.Errorf("unmapped (-want +got):\n%s", diff)
	}

	if diff := cmp.Diff([]string{"torid"}, parser.dropped); diff != "" {
		t.Errorf("dropped (-want +got):\n%s", diff)
	}
}

func TestSplicedStats(t *testing.T) {
	tests := []struct {
		class, sheet, weapon string
		optional             []string
	}{
		{"primary", "\"burston\",\"CD MS/TOX/DMG/CC/FR/CORRO/VIRAL\",\"PUNC\"\n", "burston", []string{"MS", "TOX", "DMG", "CC", "FR", "CORRO", "VIRAL"}},
		{"melee", "\"fragor\",\"CC/CD/IC/DMG/SLAMDAM\",\"\"\n", "fragor", []string{"CC", "CD", "IC", "DMG", "SLAMDAM"}},
	}
	for _, tt := range tests {
		t.Run(tt.weapon, func(t *testing.T) {
			parser := newSheetParser(tt.class)
			rolls, err := parser.parse("\"WEAPON\",\"POSITIVE STATS:\",\"NEGATIVE STATS:\"\n" + tt.sheet)
			if err != nil {
				t.Fatal(err)
			}

			roll, kept := rolls[tt.weapon]
			if !kept {
				t.Fatalf("got %s dropped, unmapped %v", tt.weapon, parser.unmapped())
			}

			if diff := cmp.Diff(tt.optional, abbrList(roll.Alternatives[0].Optional)); diff != "" {
				t.Errorf("optional (-want +got):\n%s", diff)
			}
		})
	}
}

func abbrList(stats []StatRef) []string {
	out := make([]string, 0, len(stats))
	for _, stat := range stats {
		out = append(out, stat.Abbr)
	}
	return out
}

func TestSheetWithoutColumns(t *testing.T) {
	if _, err := newSheetParser("melee").parse("\"WEAPON\",\"STUFF\"\n"); err == nil {
		t.Error("got no error for a sheet without the stat columns")
	}
}
