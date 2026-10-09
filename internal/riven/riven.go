package riven

import "time"

type Weapon struct {
	GameRef     string  `json:"game_ref"`
	Slug        string  `json:"slug"`
	Name        string  `json:"name"`
	Group       string  `json:"group"`
	RivenType   string  `json:"riven_type"`
	ModType     string  `json:"mod_type"`
	Disposition float64 `json:"disposition"`
	MasteryRank int     `json:"mastery_rank"`
}

type Attribute struct {
	Slug    string  `json:"slug"`
	GameRef string  `json:"game_ref"`
	Name    string  `json:"name"`
	Prefix  string  `json:"prefix"`
	Suffix  string  `json:"suffix"`
	Unit    *string `json:"unit"`
}

type StatRef struct {
	Abbr string   `json:"abbr"`
	Tags []string `json:"tags"`
}

type Alternative struct {
	Mandatory      []StatRef `json:"mandatory"`
	Optional       []StatRef `json:"optional"`
	OptionalNeeded int       `json:"optional_needed"`
}

type GoodRoll struct {
	Weapon       string        `json:"weapon"`
	WeaponClass  string        `json:"weapon_class"`
	Alternatives []Alternative `json:"alternatives"`
	AcceptedBad  []StatRef     `json:"accepted_bad"`
	Note         *string       `json:"note"`
}

// Document's times are 0 for parts that weren't fetched at least once.
type Document struct {
	WeaponsUpdatedAt   time.Time
	GoodRollsUpdatedAt time.Time
	Weapons            []Weapon
	Attributes         []Attribute
	GoodRolls          map[string]GoodRoll
	Unmapped           []string
}

type AuctionAttribute struct {
	URLName  string  `json:"url_name"`
	Value    float64 `json:"value"`
	Positive bool    `json:"positive"`
}

type Auction struct {
	ID            string             `json:"id"`
	BuyoutPrice   *int               `json:"buyout_price"`
	StartingPrice int                `json:"starting_price"`
	IsDirectSell  bool               `json:"is_direct_sell"`
	Name          string             `json:"name"`
	Polarity      string             `json:"polarity"`
	ModRank       int                `json:"mod_rank"`
	ReRolls       int                `json:"re_rolls"`
	MasteryLevel  int                `json:"mastery_level"`
	Attributes    []AuctionAttribute `json:"attributes"`
	OwnerName     string             `json:"owner_name"`
	OwnerStatus   string             `json:"owner_status"`
	Updated       string             `json:"updated"`
}

type WeaponAuctions struct {
	Slug      string
	UpdatedAt time.Time
	Truncated bool
	Auctions  []Auction
}
