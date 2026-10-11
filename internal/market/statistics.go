package market

import (
	"fmt"
	"slices"
	"time"

	"github.com/akshualy/api.merframe.net/internal/wfm"
)

type Statistics struct {
	Volume48h     int
	Median48h     *float64
	Volume7d      int
	Volume30d     int
	Volume90d     int
	Median7d      *float64
	Median30d     *float64
	Median90d     *float64
	Min90d        *float64
	Max90d        *float64
	DaysTraded90d int
	VolumePrev48h int
	MedianPrev48h *float64
	VolumePrev7d  int
	MedianPrev7d  *float64
	VolumePrev30d int
	MedianPrev30d *float64
}

type Day struct {
	Day    time.Time
	Volume int
	Median float64
	Min    float64
	Max    float64
	WA     float64
}

func dailySeries(entries []wfm.ClosedEntry, item wfm.Item) ([]Day, error) {
	days := []Day{}
	for _, entry := range entries {
		if !baseVariant(item, entry) {
			continue
		}

		at, err := time.Parse(time.RFC3339, entry.Datetime)
		if err != nil {
			return nil, fmt.Errorf("closed entry date: %w", err)
		}

		days = append(days, Day{
			Day: dayOf(at), Volume: entry.Volume,
			Median: entry.Median, Min: entry.MinPrice, Max: entry.MaxPrice, WA: entry.WAPrice,
		})
	}

	slices.SortFunc(days, func(a, b Day) int { return a.Day.Compare(b.Day) })
	return days, nil
}

func computeStatistics(last48h []wfm.ClosedEntry, days []Day, item wfm.Item, now time.Time) Statistics {
	week := window(days, 7, now)
	month := window(days, 30, now)
	quarter := window(days, 90, now)
	before48h := previousWindow(days, 2, now)
	beforeWeek := previousWindow(days, 7, now)
	beforeMonth := previousWindow(days, 30, now)
	s := Statistics{
		Median48h:     closedMedian(last48h, item),
		Volume7d:      volume(week),
		Volume30d:     volume(month),
		Volume90d:     volume(quarter),
		Median7d:      median(week),
		Median30d:     median(month),
		Median90d:     median(quarter),
		DaysTraded90d: len(quarter),
		VolumePrev48h: volume(before48h),
		MedianPrev48h: median(before48h),
		VolumePrev7d:  volume(beforeWeek),
		MedianPrev7d:  median(beforeWeek),
		VolumePrev30d: volume(beforeMonth),
		MedianPrev30d: median(beforeMonth),
	}
	for _, entry := range last48h {
		if baseVariant(item, entry) {
			s.Volume48h += entry.Volume
		}
	}

	for i, day := range quarter {
		if i == 0 || day.Min < *s.Min90d {
			s.Min90d = new(day.Min)
		}

		if i == 0 || day.Max > *s.Max90d {
			s.Max90d = new(day.Max)
		}
	}
	return s
}

// baseVariant keeps the closed trades of the variant the quote describes: rank 0 or unranked, base subtype or none.
func baseVariant(item wfm.Item, entry wfm.ClosedEntry) bool {
	return baseRank(entry.ModRank) && baseSubtype(item, entry.Subtype)
}

func closedMedian(entries []wfm.ClosedEntry, item wfm.Item) *float64 {
	var newest *wfm.ClosedEntry
	for i := range entries {
		entry := &entries[i]
		if baseVariant(item, *entry) && (newest == nil || entry.Datetime > newest.Datetime) {
			newest = entry
		}
	}

	if newest == nil {
		return nil
	}
	return new(newest.Median)
}

func window(days []Day, length int, now time.Time) []Day {
	since := dayOf(now.UTC().AddDate(0, 0, -length))
	var out []Day
	for _, day := range days {
		if !day.Day.Before(since) && day.Volume > 0 {
			out = append(out, day)
		}
	}
	return out
}

func previousWindow(days []Day, length int, now time.Time) []Day {
	until := dayOf(now.UTC().AddDate(0, 0, -length))
	var out []Day
	for _, day := range window(days, 2*length, now) {
		if day.Day.Before(until) {
			out = append(out, day)
		}
	}
	return out
}

func dayOf(t time.Time) time.Time {
	y, m, d := t.UTC().Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func volume(days []Day) int {
	total := 0
	for _, day := range days {
		total += day.Volume
	}
	return total
}

func median(days []Day) *float64 {
	if len(days) == 0 {
		return nil
	}

	values := make([]float64, len(days))
	for i, day := range days {
		values[i] = day.Median
	}

	slices.Sort(values)
	mid := len(values) / 2
	if len(values)%2 == 1 {
		return new(values[mid])
	}
	return new((values[mid-1] + values[mid]) / 2)
}
