package riven

import (
	"encoding/csv"
	"errors"
	"regexp"
	"slices"
	"strings"
)

const (
	positivesPerRoll = 3
	positiveHeader   = "POSITIVE STATS:"
	negativeHeader   = "NEGATIVE STATS:"
	notesHeader      = "Notes:"
	meleeClass       = "melee"
)

var elements = []string{
	"WeaponFreezeDamageMod",
	"WeaponFireDamageMod",
	"WeaponToxinDamageMod",
	"WeaponElectricityDamageMod",
}

var gunStats = map[string][]string{
	"CD":      {"WeaponCritDamageMod"},
	"CC":      {"WeaponCritChanceMod"},
	"DMG":     {"WeaponDamageAmountMod"},
	"MS":      {"WeaponFireIterationsMod"},
	"FR":      {"WeaponFireRateMod"},
	"RLS":     {"WeaponReloadSpeedMod"},
	"TOX":     {"WeaponToxinDamageMod"},
	"HEAT":    {"WeaponFireDamageMod"},
	"COLD":    {"WeaponFreezeDamageMod"},
	"ELEC":    {"WeaponElectricityDamageMod"},
	"ELEMENT": elements,
	"DTC":     {"WeaponFactionDamageCorpus"},
	"DTI":     {"WeaponFactionDamageInfested"},
	"DTG":     {"WeaponFactionDamageGrineer"},
	"PUNC":    {"WeaponArmorPiercingDamageMod"},
	"IMP":     {"WeaponImpactDamageMod"},
	"SLASH":   {"WeaponSlashDamageMod"},
	"MAG":     {"WeaponClipMaxMod"},
	"AMMO":    {"WeaponAmmoMaxMod"},
	"REC":     {"WeaponRecoilReductionMod"},
	"RECOIL":  {"WeaponRecoilReductionMod"},
	"ZOOM":    {"WeaponZoomFovMod"},
	"SC":      {"WeaponStunChanceMod"},
	"SD":      {"WeaponProcTimeMod"},
	"PT":      {"WeaponPunctureDepthMod"},
	"PFS":     {"WeaponProjectileSpeedMod"},
	"CORRO":   {"WeaponCorrosiveDamageMod"},
	"VIRAL":   {"WeaponViralDamageMod"},
	"DTO":     {"WeaponFactionDamageOrokin"},
	"DTT":     {"WeaponFactionDamageTechrot"},
	"DTS":     {"WeaponFactionDamageScaldra"},
	"WEAKDMG": {"WeaponWeakpointDamage"},
	"SDMG":    {"WeaponStatusDamage"},
}

var meleeStats = map[string][]string{
	"CD":      {"WeaponCritDamageMod"},
	"CC":      {"WeaponCritChanceMod"},
	"DMG":     {"WeaponMeleeDamageMod"},
	"AS":      {"WeaponFireRateMod"},
	"TOX":     {"WeaponToxinDamageMod"},
	"HEAT":    {"WeaponFireDamageMod"},
	"COLD":    {"WeaponFreezeDamageMod"},
	"ELEC":    {"WeaponElectricityDamageMod"},
	"ELEMENT": elements,
	"DTC":     {"WeaponMeleeFactionDamageCorpus"},
	"DTI":     {"WeaponMeleeFactionDamageInfested"},
	"DTG":     {"WeaponMeleeFactionDamageGrineer"},
	"PUNC":    {"WeaponArmorPiercingDamageMod"},
	"IMP":     {"WeaponImpactDamageMod"},
	"SLASH":   {"WeaponSlashDamageMod"},
	"SC":      {"WeaponStunChanceMod"},
	"SD":      {"WeaponProcTimeMod"},
	"RANGE":   {"WeaponMeleeRangeIncMod"},
	"IC":      {"WeaponMeleeComboInitialBonusMod"},
	"EFF":     {"WeaponMeleeComboEfficiencyMod"},
	"SLIDE":   {"SlideAttackCritChanceMod"},
	"FIN":     {"WeaponMeleeFinisherDamageMod"},
	"CDUR":    {"ComboDurationMod"},
	"CORRO":   {"WeaponCorrosiveDamageMod"},
	"DTO":     {"WeaponFactionDamageOrokin"},
	"DTT":     {"WeaponFactionDamageTechrot"},
	"DTS":     {"WeaponFactionDamageScaldra"},
	"SLAMDAM": {"WeaponMeleeSlamDamageMod"},
}

var alternativeSplit = regexp.MustCompile(`(?i)\s+or\s+`)

var errMissingColumns = errors.New("the sheet doesn't have a positive or negative stats column")

type sheetParser struct {
	class   string
	stats   map[string][]string
	missing map[string]bool
	dropped []string
}

func newSheetParser(weaponClass string) *sheetParser {
	stats := gunStats
	if weaponClass == meleeClass {
		stats = meleeStats
	}
	return &sheetParser{class: weaponClass, stats: stats, missing: map[string]bool{}}
}

// unmapped lists abbreviations the sheet used that weren't known as class:ABBR.
func (p *sheetParser) unmapped() []string {
	out := make([]string, 0, len(p.missing))
	for abbr := range p.missing {
		out = append(out, abbr)
	}

	slices.Sort(out)
	return out
}

func (p *sheetParser) parse(text string) (map[string]GoodRoll, error) {
	reader := csv.NewReader(strings.NewReader(text))
	reader.FieldsPerRecord = -1
	rows, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}

	if len(rows) == 0 {
		return map[string]GoodRoll{}, nil
	}

	header := rows[0]
	positive := slices.Index(header, positiveHeader)
	negative := slices.Index(header, negativeHeader)
	notes := slices.Index(header, notesHeader)

	if positive < 0 || negative < 0 {
		return nil, errMissingColumns
	}

	rolls := map[string]GoodRoll{}
	for _, row := range rows[1:] {
		if len(row) <= negative {
			continue
		}

		weapon := strings.ToLower(strings.TrimSpace(row[0]))
		if weapon == "" {
			continue
		}

		var alternatives []Alternative
		for _, part := range alternativeSplit.Split(strings.TrimSpace(row[positive]), -1) {
			if alternative, ok := p.alternative(part); ok {
				alternatives = append(alternatives, alternative)
			}
		}

		alternatives = collapsed(alternatives)
		if len(alternatives) == 0 {
			p.dropped = append(p.dropped, weapon)
			continue
		}

		roll := GoodRoll{
			Weapon:       weapon,
			WeaponClass:  p.class,
			Alternatives: alternatives,
			AcceptedBad:  []StatRef{},
		}
		if bad, ok := p.statsOf(strings.TrimSpace(row[negative])); ok {
			roll.AcceptedBad = bad
		}

		if notes >= 0 && len(row) > notes {
			if note := strings.TrimSpace(row[notes]); note != "" {
				roll.Note = &note
			}
		}

		rolls[weapon] = roll
	}
	return rolls, nil
}

func (p *sheetParser) stat(abbr string) (StatRef, bool) {
	abbr = strings.ToUpper(abbr)
	tags, ok := p.stats[abbr]
	if !ok {
		p.missing[p.class+":"+abbr] = true
		return StatRef{}, false
	}
	return StatRef{Abbr: abbr, Tags: tags}, true
}

func (p *sheetParser) statsOf(text string) ([]StatRef, bool) {
	refs := []StatRef{}
	known := true
	for part := range strings.SplitSeq(text, "/") {
		if strings.TrimSpace(part) == "" {
			continue
		}

		ref, ok := p.stat(part)
		if !ok {
			known = false
			continue
		}

		refs = append(refs, ref)
	}
	return refs, known
}

func (p *sheetParser) alternative(text string) (Alternative, bool) {
	mandatory := []StatRef{}
	optional := []StatRef{}
	for token := range strings.FieldsSeq(text) {
		refs, ok := p.statsOf(token)
		if !ok {
			return Alternative{}, false
		}

		if strings.Contains(token, "/") {
			optional = append(optional, refs...)
		} else {
			mandatory = append(mandatory, refs...)
		}
	}
	if len(mandatory) == 0 && len(optional) == 0 {
		return Alternative{}, false
	}

	needed := min(len(optional), max(0, positivesPerRoll-len(mandatory)))
	return Alternative{Mandatory: mandatory, Optional: optional, OptionalNeeded: needed}, true
}

// collapsed merges "TOX DTC or TOX DTG or TOX DTI" into "TOX plus any one of DTC, DTG, DTI".
func collapsed(alternatives []Alternative) []Alternative {
	var merged []Alternative
	for _, alternative := range alternatives {
		if len(alternative.Optional) > 0 {
			merged = append(merged, alternative)
			continue
		}

		mine := abbrs(alternative.Mandatory)
		joined := false
		for i, kept := range merged {
			if (kept.OptionalNeeded != 0 && kept.OptionalNeeded != 1) || overlaps(kept.Optional, mine) {
				continue
			}

			shared := abbrs(kept.Mandatory)
			if countExcept(mine, shared) != 1 || countExcept(shared, mine) > 1 {
				continue
			}

			common, keptApart := []StatRef{}, []StatRef{}
			for _, stat := range kept.Mandatory {
				if mine[stat.Abbr] {
					common = append(common, stat)
				} else {
					keptApart = append(keptApart, stat)
				}
			}
			apart := []StatRef{}
			for _, stat := range alternative.Mandatory {
				if !shared[stat.Abbr] {
					apart = append(apart, stat)
				}
			}

			optional := slices.Concat(kept.Optional, keptApart, apart)
			merged[i] = Alternative{Mandatory: common, Optional: optional, OptionalNeeded: 1}
			joined = true
			break
		}
		if !joined {
			merged = append(merged, alternative)
		}
	}
	return merged
}

func abbrs(stats []StatRef) map[string]bool {
	set := make(map[string]bool, len(stats))
	for _, stat := range stats {
		set[stat.Abbr] = true
	}
	return set
}

func overlaps(stats []StatRef, set map[string]bool) bool {
	for _, stat := range stats {
		if set[stat.Abbr] {
			return true
		}
	}
	return false
}

func countExcept(a, b map[string]bool) int {
	n := 0

	for abbr := range a {
		if !b[abbr] {
			n++
		}
	}
	return n
}
