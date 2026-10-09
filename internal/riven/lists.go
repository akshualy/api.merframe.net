package riven

import (
	"slices"
	"strings"

	"github.com/akshualy/api.merframe.net/internal/wfm"
)

const randomizedMods = "/Lotus/Upgrades/Mods/Randomized/"

var modTypes = map[string]string{
	"rifle":   "LotusRifleRandomModRare",
	"shotgun": "LotusShotgunRandomModRare",
	"pistol":  "LotusPistolRandomModRare",
	"melee":   "PlayerMeleeWeaponRandomModRare",
	"kitgun":  "LotusModularPistolRandomModRare",
	"zaw":     "LotusModularMeleeRandomModRare",
}

// weapons drops entries with riven types without mod types. Archguns are mapped by group.
func weapons(entries []wfm.RivenWeapon) []Weapon {
	out := []Weapon{}
	for _, entry := range entries {
		modType, ok := modTypeOf(entry)
		if !ok {
			continue
		}

		out = append(out, Weapon{
			GameRef:     entry.GameRef,
			Slug:        entry.Slug,
			Name:        entry.I18n.En.Name,
			Group:       entry.Group,
			RivenType:   entry.RivenType,
			ModType:     modType,
			Disposition: entry.Disposition,
			MasteryRank: entry.ReqMasteryRank,
		})
	}

	slices.SortFunc(out, func(a, b Weapon) int { return strings.Compare(a.Name, b.Name) })
	return out
}

func modTypeOf(entry wfm.RivenWeapon) (string, bool) {
	if entry.Group == "archgun" {
		return randomizedMods + "LotusArchgunRandomModRare", true
	}

	name, ok := modTypes[entry.RivenType]
	return randomizedMods + name, ok
}

func attributes(entries []wfm.RivenAttribute) []Attribute {
	out := make([]Attribute, len(entries))
	for i, entry := range entries {
		out[i] = Attribute{
			Slug:    entry.Slug,
			GameRef: entry.GameRef,
			Name:    entry.I18n.En.Name,
			Prefix:  entry.Prefix,
			Suffix:  entry.Suffix,
			Unit:    entry.Unit,
		}
	}

	slices.SortFunc(out, func(a, b Attribute) int { return strings.Compare(a.Slug, b.Slug) })
	return out
}

func openAuctions(raw []wfm.Auction) []Auction {
	out := make([]Auction, 0, len(raw))
	for _, entry := range raw {
		if !entry.Visible || entry.Closed || entry.Private {
			continue
		}

		stats := make([]AuctionAttribute, len(entry.Item.Attributes))
		for i, attribute := range entry.Item.Attributes {
			stats[i] = AuctionAttribute(attribute)
		}

		out = append(out, Auction{
			ID:            entry.ID,
			BuyoutPrice:   entry.BuyoutPrice,
			StartingPrice: entry.StartingPrice,
			IsDirectSell:  entry.IsDirectSell,
			Name:          entry.Item.Name,
			Polarity:      entry.Item.Polarity,
			ModRank:       entry.Item.ModRank,
			ReRolls:       entry.Item.ReRolls,
			MasteryLevel:  entry.Item.MasteryLevel,
			Attributes:    stats,
			OwnerName:     entry.Owner.IngameName,
			OwnerStatus:   entry.Owner.Status,
			Updated:       entry.Updated,
		})
	}
	return out
}
