package riven

import (
	"context"
	"log/slog"
	"strconv"
	"testing"
	"testing/synctest"
	"time"

	"github.com/akshualy/api.merframe.net/internal/wfm"
)

type fakeAuctionSource struct {
	searches []wfm.AuctionSearch
	perQuery func(wfm.AuctionSearch) []wfm.Auction
}

func (f *fakeAuctionSource) SearchAuctions(_ context.Context, search wfm.AuctionSearch) ([]wfm.Auction, error) {
	f.searches = append(f.searches, search)
	return f.perQuery(search), nil
}

func listing(id string) wfm.Auction {
	a := wfm.Auction{ID: id, Visible: true}
	a.Item.Attributes = []wfm.AuctionAttribute{}
	return a
}

func listings(prefix string, n int) []wfm.Auction {
	out := make([]wfm.Auction, n)
	for i := range out {
		out[i] = listing(prefix + string(rune('a'+i%26)) + string(rune('a'+i/26)))
	}
	return out
}

func TestCrawlerSplit(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		source := &fakeAuctionSource{perQuery: func(s wfm.AuctionSearch) []wfm.Auction {
			switch {
			case s.Weapon == "quiet":
				return []wfm.Auction{listing("q1"), {ID: "hidden", Visible: false}}
			case s.Polarity == "":
				return listings("all", searchCap)
			case s.Polarity == "madurai" && s.ReRollsMin == nil:
				return listings("mad", searchCap)
			case s.Polarity == "madurai" && *s.ReRollsMin == 50:
				return listings("m50", searchCap)
			case s.Polarity == "madurai":
				return listings("m"+strconv.Itoa(*s.ReRollsMin), 10)
			default:
				return listings(s.Polarity, 3)
			}
		}}

		store := newFakeStore()
		store.data = Document{Weapons: []Weapon{{Slug: "popular"}, {Slug: "quiet"}}}
		publisher := &fakePublisher{}
		c := NewCrawler(source, store, publisher, 6*time.Second, 24*time.Hour, slog.New(slog.DiscardHandler))
		runUntilCancelled(t, c.Run, time.Hour)

		if len(source.searches) != 1+4+5+1 {
			t.Errorf("%d searches, want whole + 4 polarities + 5 madurai buckets + quiet", len(source.searches))
		}

		popular := publisher.auctions["popular"]
		if !popular.Truncated {
			t.Error("got truncated false, want true with a bucket at the cap")
		}

		if got, want := len(popular.Auctions), 3*searchCap+4*10+3*3; got != want {
			t.Errorf("kept %d listings for popular, want %d", got, want)
		}

		quiet := publisher.auctions["quiet"]
		if len(quiet.Auctions) != 1 || quiet.Truncated {
			t.Errorf("quiet = %+v", quiet)
		}

		if len(publisher.auctions) != 2 {
			t.Errorf("published %v", publisher.auctions)
		}
	})
}

func TestCrawlerPace(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		source := &fakeAuctionSource{perQuery: func(wfm.AuctionSearch) []wfm.Auction { return nil }}
		store := newFakeStore()
		store.data = Document{Weapons: []Weapon{{Slug: "a"}, {Slug: "b"}, {Slug: "c"}}}
		c := NewCrawler(source, store, &fakePublisher{}, 6*time.Second, 24*time.Hour, slog.New(slog.DiscardHandler))
		runUntilCancelled(t, c.Run, 11*time.Second)

		if len(source.searches) != 2 {
			t.Errorf("%d searches in 11s at one per 6s, want 2", len(source.searches))
		}
	})
}
