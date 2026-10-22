package riven

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"strconv"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/akshualy/api.merframe.net/internal/wfm"
)

type fakeAuctionSource struct {
	mu       sync.Mutex
	searches []wfm.AuctionSearch
	perQuery func(wfm.AuctionSearch) []wfm.Auction
	err      error
}

func (f *fakeAuctionSource) SearchAuctions(_ context.Context, search wfm.AuctionSearch) ([]wfm.Auction, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.searches = append(f.searches, search)
	return f.perQuery(search), f.err
}

func (f *fakeAuctionSource) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.searches)
}

func (f *fakeAuctionSource) last() wfm.AuctionSearch {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.searches[len(f.searches)-1]
}

// fakeDemand applies the due rule and the order of the store's query.
type fakeDemand struct {
	mu          sync.Mutex
	requestedAt map[string]time.Time
	crawledAt   map[string]time.Time
}

func newFakeDemand(slugs ...string) *fakeDemand {
	d := &fakeDemand{requestedAt: map[string]time.Time{}, crawledAt: map[string]time.Time{}}
	for _, slug := range slugs {
		d.request(slug)
	}
	return d
}

func (f *fakeDemand) request(slug string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.requestedAt[slug] = time.Now()
}

func (f *fakeDemand) DueAuctions(_ context.Context, before time.Time) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var due []string
	for slug, requestedAt := range f.requestedAt {
		crawledAt, crawled := f.crawledAt[slug]
		if !crawled || (crawledAt.Before(requestedAt) && crawledAt.Before(before)) {
			due = append(due, slug)
		}
	}

	slices.SortFunc(due, func(a, b string) int {
		if c := f.crawledAt[a].Compare(f.crawledAt[b]); c != 0 {
			return c
		}
		return f.requestedAt[a].Compare(f.requestedAt[b])
	})
	return due, nil
}

func (f *fakeDemand) MarkAuctionsCrawled(_ context.Context, slug string, at time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.crawledAt[slug] = at
	return nil
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

		publisher := &fakePublisher{}
		c := NewCrawler(source, newFakeDemand("popular", "quiet"), publisher, 6*time.Second, time.Hour, slog.New(slog.DiscardHandler))
		runUntilCancelled(t, c.Run, time.Hour)

		if source.count() != 1+4+5+1 {
			t.Errorf("%d searches, want whole + 4 polarities + 5 madurai buckets + quiet", source.count())
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
		c := NewCrawler(source, newFakeDemand("a", "b", "c"), &fakePublisher{}, 6*time.Second, time.Hour, slog.New(slog.DiscardHandler))
		runUntilCancelled(t, c.Run, 11*time.Second)

		if source.count() != 2 {
			t.Errorf("%d searches in 11s at one per 6s, want 2", source.count())
		}
	})
}

func TestCrawlerFollowsDemand(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		source := &fakeAuctionSource{perQuery: func(wfm.AuctionSearch) []wfm.Auction { return nil }}
		demand := newFakeDemand("torid")
		c := NewCrawler(source, demand, &fakePublisher{}, 6*time.Second, time.Hour, slog.New(slog.DiscardHandler))
		stop := start(t, c.Run)

		time.Sleep(30 * time.Minute)
		synctest.Wait()
		if source.count() != 1 {
			t.Errorf("%d searches after one request, want 1", source.count())
		}

		demand.request("torid")
		time.Sleep(20 * time.Minute)
		synctest.Wait()
		if source.count() != 1 {
			t.Errorf("%d searches, want a second request inside the interval to wait", source.count())
		}

		time.Sleep(20 * time.Minute)
		synctest.Wait()
		if source.count() != 2 {
			t.Errorf("%d searches, want the second request served once the interval passed", source.count())
		}

		time.Sleep(2 * time.Hour)
		synctest.Wait()
		if source.count() != 2 {
			t.Errorf("%d searches, want no crawl without a new request", source.count())
		}

		stop()
	})
}

func TestCrawlerFailureWaitsLikeASuccess(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		source := &fakeAuctionSource{perQuery: func(wfm.AuctionSearch) []wfm.Auction { return nil }, err: errors.New("boom")}
		demand := newFakeDemand("torid")
		c := NewCrawler(source, demand, &fakePublisher{}, 6*time.Second, time.Hour, slog.New(slog.DiscardHandler))
		stop := start(t, c.Run)

		time.Sleep(30 * time.Minute)
		synctest.Wait()
		if source.count() != 1 {
			t.Errorf("%d searches, want one failed crawl and no retry inside the interval", source.count())
		}

		demand.request("torid")
		time.Sleep(time.Hour)
		synctest.Wait()
		if source.count() != 2 {
			t.Errorf("%d searches, want the next request after the interval to crawl again", source.count())
		}

		stop()
	})
}

func TestCrawlerServesNewWeaponFirst(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		source := &fakeAuctionSource{perQuery: func(wfm.AuctionSearch) []wfm.Auction { return nil }}
		demand := newFakeDemand("a", "b", "c")
		c := NewCrawler(source, demand, &fakePublisher{}, 6*time.Second, time.Hour, slog.New(slog.DiscardHandler))
		stop := start(t, c.Run)

		time.Sleep(time.Minute)
		synctest.Wait()
		for _, slug := range []string{"a", "b", "c"} {
			demand.request(slug)
		}

		time.Sleep(time.Hour + time.Second)
		synctest.Wait()
		demand.request("new")
		time.Sleep(8 * time.Second)
		synctest.Wait()
		if searched := source.last().Weapon; searched != "new" {
			t.Errorf("crawled %q, want a never crawled weapon ahead of the stale ones", searched)
		}

		stop()
	})
}
