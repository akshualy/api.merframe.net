package riven

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"time"

	"github.com/akshualy/api.merframe.net/internal/throttle"
	"github.com/akshualy/api.merframe.net/internal/wfm"
)

type Source interface {
	RivenWeapons(ctx context.Context) ([]wfm.RivenWeapon, error)
	RivenAttributes(ctx context.Context) ([]wfm.RivenAttribute, error)
}

type Sheet interface {
	Tab(ctx context.Context, name string) (string, error)
}

type Store interface {
	Riven(ctx context.Context) (Document, error)
	SaveRiven(ctx context.Context, d Document) error
}

type Publisher interface {
	Riven(ctx context.Context, d Document) error
	Auctions(ctx context.Context, w WeaponAuctions) error
}

const refreshCheckPause = time.Hour

var goodRollTabs = []struct{ class, tab string }{
	{"primary", "Primary"},
	{"secondary", "Secondary"},
	{"melee", "Melee"},
	{"archgun", "Archgun"},
}

var errEmptyList = errors.New("the list came back empty")

type Refresher struct {
	source    Source
	sheet     Sheet
	store     Store
	publisher Publisher
	interval  time.Duration
	log       *slog.Logger
	data      Document
}

func NewRefresher(source Source, sheet Sheet, store Store, publisher Publisher, interval time.Duration, log *slog.Logger) *Refresher {
	return &Refresher{source: source, sheet: sheet, store: store, publisher: publisher, interval: interval, log: log}
}

func (r *Refresher) Run(ctx context.Context) error {
	data, err := r.store.Riven(ctx)
	if err != nil {
		return fmt.Errorf("loading the riven document: %w", err)
	}

	r.data = data

	for {
		changed := false
		if r.stale(r.data.WeaponsUpdatedAt) {
			if err := r.refreshLists(ctx); err != nil {
				if ctx.Err() != nil {
					return ctx.Err()
				}

				r.log.Warn("refreshing the riven weapon list failed", "error", err)
			} else {
				changed = true
			}
		}
		if r.stale(r.data.GoodRollsUpdatedAt) {
			if err := r.refreshGoodRolls(ctx); err != nil {
				if ctx.Err() != nil {
					return ctx.Err()
				}

				r.log.Warn("refreshing the good rolls sheet failed", "error", err)
			} else {
				changed = true
			}
		}
		if changed {
			if err := r.store.SaveRiven(ctx, r.data); err != nil {
				return fmt.Errorf("saving the riven document: %w", err)
			}

			if err := r.publisher.Riven(ctx, r.data); err != nil {
				return fmt.Errorf("publishing the riven document: %w", err)
			}
		}

		if err := throttle.Sleep(ctx, refreshCheckPause); err != nil {
			return err
		}
	}
}

func (r *Refresher) stale(updatedAt time.Time) bool {
	return time.Since(updatedAt) > r.interval
}

func (r *Refresher) refreshLists(ctx context.Context) error {
	rawWeapons, err := r.source.RivenWeapons(ctx)
	if err != nil {
		return err
	}

	rawAttributes, err := r.source.RivenAttributes(ctx)
	if err != nil {
		return err
	}

	fetchedWeapons := weapons(rawWeapons)
	if len(fetchedWeapons) == 0 {
		return fmt.Errorf("weapons: %w", errEmptyList)
	}

	fetchedAttributes := attributes(rawAttributes)
	if len(fetchedAttributes) == 0 {
		return fmt.Errorf("attributes: %w", errEmptyList)
	}

	r.data.Weapons = fetchedWeapons
	r.data.Attributes = fetchedAttributes
	r.data.WeaponsUpdatedAt = time.Now()
	r.log.Info("riven lists refreshed", "weapons", len(fetchedWeapons), "attributes", len(fetchedAttributes))
	return nil
}

func (r *Refresher) refreshGoodRolls(ctx context.Context) error {
	rolls := map[string]GoodRoll{}
	unmapped := []string{}
	dropped := []string{}
	for _, tab := range goodRollTabs {
		text, err := r.sheet.Tab(ctx, tab.tab)
		if err != nil {
			return err
		}

		parser := newSheetParser(tab.class)
		parsed, err := parser.parse(text)
		if err != nil {
			return fmt.Errorf("tab %s: %w", tab.tab, err)
		}

		for weapon, roll := range parsed {
			rolls[weapon] = roll
		}

		unmapped = append(unmapped, parser.unmapped()...)
		dropped = append(dropped, parser.dropped...)
	}

	if len(rolls) == 0 {
		return fmt.Errorf("good rolls: %w", errEmptyList)
	}

	slices.Sort(unmapped)
	r.data.GoodRolls = rolls
	r.data.Unmapped = unmapped
	r.data.GoodRollsUpdatedAt = time.Now()
	r.log.Info("good rolls refreshed", "weapons", len(rolls), "unmapped", unmapped, "dropped", dropped)
	return nil
}
