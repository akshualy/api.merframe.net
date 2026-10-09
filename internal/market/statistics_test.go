package market

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/akshualy/api.merframe.net/internal/wfm"
)

type expectedStatistics struct {
	Now        string `json:"now"`
	Statistics struct {
		Volume48h     int      `json:"volume_48h"`
		Median48h     *float64 `json:"median_48h"`
		Volume7d      int      `json:"volume_7d"`
		Volume30d     int      `json:"volume_30d"`
		Volume90d     int      `json:"volume_90d"`
		Median7d      *float64 `json:"median_7d"`
		Median30d     *float64 `json:"median_30d"`
		Median90d     *float64 `json:"median_90d"`
		Min90d        *float64 `json:"min_90d"`
		Max90d        *float64 `json:"max_90d"`
		DaysTraded90d int      `json:"days_traded_90d"`
		VolumePrev48h int      `json:"volume_prev_48h"`
		MedianPrev48h *float64 `json:"median_prev_48h"`
		VolumePrev7d  int      `json:"volume_prev_7d"`
		MedianPrev7d  *float64 `json:"median_prev_7d"`
		VolumePrev30d int      `json:"volume_prev_30d"`
		MedianPrev30d *float64 `json:"median_prev_30d"`
	} `json:"statistics"`
	Days []struct {
		Day    string  `json:"day"`
		Volume int     `json:"volume"`
		Median float64 `json:"median"`
		Min    float64 `json:"min"`
		Max    float64 `json:"max"`
		WA     float64 `json:"wa"`
	} `json:"days"`
}

func TestStatistics(t *testing.T) {
	var expected expectedStatistics
	readJSON(t, "expected", "statistics.json", &expected)
	var envelope struct {
		Payload struct {
			Closed wfm.ClosedStatistics `json:"statistics_closed"`
		} `json:"payload"`
	}
	readJSON(t, "wfm", "statistics.json", &envelope)

	now, err := time.Parse(time.RFC3339, expected.Now)
	if err != nil {
		t.Fatal(err)
	}

	days, err := dailySeries(envelope.Payload.Closed.Last90Days)
	if err != nil {
		t.Fatal(err)
	}

	if len(days) != len(expected.Days) {
		t.Fatalf("kept %d days, want %d", len(days), len(expected.Days))
	}

	for i, want := range expected.Days {
		got := days[i]
		if got.Day.Format("2006-01-02") != want.Day || got.Volume != want.Volume || got.Median != want.Median || got.Min != want.Min || got.Max != want.Max || got.WA != want.WA {
			t.Errorf("day %d = %+v, want %+v", i, got, want)
		}
	}

	got := computeStatistics(envelope.Payload.Closed.Last48Hours, days, now)
	if diff := cmp.Diff(Statistics(expected.Statistics), got); diff != "" {
		t.Errorf("statistics (-want +got):\n%s", diff)
	}
}

func TestStatisticsEmpty(t *testing.T) {
	got := computeStatistics(nil, nil, time.Now())
	if diff := cmp.Diff(Statistics{}, got); diff != "" {
		t.Errorf("statistics of nothing (-want +got):\n%s", diff)
	}
}

func TestDailySeriesBadDate(t *testing.T) {
	if _, err := dailySeries([]wfm.ClosedEntry{{Datetime: "yesterday"}}); err == nil {
		t.Error("dailySeries accepted an unparsable date, want an error")
	}
}
