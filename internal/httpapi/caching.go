package httpapi

import "github.com/akshualy/api.merframe.net/internal/blob"

// Edge directives are not in Cache-Control because an s-maxage there would switch the stale ones off.
var (
	pricesCaching = blob.Caching{
		Edge:   "max-age=60, stale-while-revalidate=300, stale-if-error=86400",
		Client: "public, max-age=0",
	}
	itemCaching = blob.Caching{
		Edge:   "max-age=60",
		Client: "public, max-age=60",
	}
	catalogCaching = blob.Caching{
		Edge:   "max-age=3600, stale-while-revalidate=86400, stale-if-error=86400",
		Client: "public, max-age=3600",
	}
	rivenCaching    = catalogCaching
	auctionsCaching = blob.Caching{
		Edge:   "max-age=300, stale-while-revalidate=3600, stale-if-error=86400",
		Client: "public, max-age=300",
	}
)

const noStore = "no-store"
