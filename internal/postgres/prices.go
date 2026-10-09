package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/akshualy/api.merframe.net/internal/market"
	"github.com/akshualy/api.merframe.net/internal/postgres/sqlc"
)

func (s *Store) SavePrice(ctx context.Context, slug string, price market.Price, days []market.Day) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.q.WithTx(tx)

		if err := q.UpsertPrice(ctx, priceParams(slug, price)); err != nil {
			return fmt.Errorf("saving the price of %s: %w", slug, err)
		}

		if days == nil {
			return nil
		}

		if err := q.DeleteHistory(ctx, slug); err != nil {
			return fmt.Errorf("clearing the history of %s: %w", slug, err)
		}

		rows := make([]sqlc.InsertHistoryParams, len(days))
		for i, day := range days {
			rows[i] = sqlc.InsertHistoryParams{
				Slug: slug, Day: day.Day, Volume: day.Volume,
				Median: day.Median, Min: day.Min, Max: day.Max, Wa: day.WA,
			}
		}

		if _, err := q.InsertHistory(ctx, rows); err != nil {
			return fmt.Errorf("saving the history of %s: %w", slug, err)
		}
		return nil
	})
}

func (s *Store) Price(ctx context.Context, slug string) (market.Price, error) {
	row, err := s.q.GetPrice(ctx, slug)
	if errors.Is(err, pgx.ErrNoRows) {
		return market.Price{}, ErrMissing
	}

	if err != nil {
		return market.Price{}, err
	}
	return priceOf(row), nil
}

func (s *Store) Prices(ctx context.Context) (map[string]market.Price, error) {
	rows, err := s.q.ListPrices(ctx)
	if err != nil {
		return nil, err
	}

	prices := make(map[string]market.Price, len(rows))
	for _, row := range rows {
		prices[row.Slug] = priceOf(row)
	}
	return prices, nil
}

func (s *Store) History(ctx context.Context, slug string) ([]market.Day, error) {
	rows, err := s.q.ListHistory(ctx, slug)
	if err != nil {
		return nil, err
	}

	days := make([]market.Day, len(rows))
	for i, row := range rows {
		days[i] = market.Day{
			Day: row.Day, Volume: row.Volume,
			Median: row.Median, Min: row.Min, Max: row.Max, WA: row.Wa,
		}
	}
	return days, nil
}

func priceParams(slug string, p market.Price) sqlc.UpsertPriceParams {
	return sqlc.UpsertPriceParams{
		Slug:          slug,
		SellR0:        p.SellR0,
		SellMax:       p.SellMax,
		SellR0Online:  p.SellR0Online,
		SellMaxOnline: p.SellMaxOnline,
		BuyR0:         p.BuyR0,
		BuyMax:        p.BuyMax,
		SellCount:     p.SellCount,
		BuyCount:      p.BuyCount,
		Volume48h:     p.Volume48h,
		Median48h:     p.Median48h,
		Volume7d:      p.Volume7d,
		Volume30d:     p.Volume30d,
		Volume90d:     p.Volume90d,
		Median7d:      p.Median7d,
		Median30d:     p.Median30d,
		Median90d:     p.Median90d,
		Min90d:        p.Min90d,
		Max90d:        p.Max90d,
		DaysTraded90d: p.DaysTraded90d,
		VolumePrev48h: p.VolumePrev48h,
		MedianPrev48h: p.MedianPrev48h,
		VolumePrev7d:  p.VolumePrev7d,
		MedianPrev7d:  p.MedianPrev7d,
		VolumePrev30d: p.VolumePrev30d,
		MedianPrev30d: p.MedianPrev30d,
		Tier:          string(p.Tier),
		UpdatedAt:     p.UpdatedAt,
		StatisticsAt:  p.StatisticsAt,
	}
}

func priceOf(r sqlc.Price) market.Price {
	return market.Price{
		Quote: market.Quote{
			SellR0:        r.SellR0,
			SellMax:       r.SellMax,
			SellR0Online:  r.SellR0Online,
			SellMaxOnline: r.SellMaxOnline,
			BuyR0:         r.BuyR0,
			BuyMax:        r.BuyMax,
			SellCount:     r.SellCount,
			BuyCount:      r.BuyCount,
		},
		Statistics: market.Statistics{
			Volume48h:     r.Volume48h,
			Median48h:     r.Median48h,
			Volume7d:      r.Volume7d,
			Volume30d:     r.Volume30d,
			Volume90d:     r.Volume90d,
			Median7d:      r.Median7d,
			Median30d:     r.Median30d,
			Median90d:     r.Median90d,
			Min90d:        r.Min90d,
			Max90d:        r.Max90d,
			DaysTraded90d: r.DaysTraded90d,
			VolumePrev48h: r.VolumePrev48h,
			MedianPrev48h: r.MedianPrev48h,
			VolumePrev7d:  r.VolumePrev7d,
			MedianPrev7d:  r.MedianPrev7d,
			VolumePrev30d: r.VolumePrev30d,
			MedianPrev30d: r.MedianPrev30d,
		},
		Tier:         market.Tier(r.Tier),
		UpdatedAt:    r.UpdatedAt,
		StatisticsAt: r.StatisticsAt,
	}
}
