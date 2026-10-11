package wfm

import "encoding/json"

type Item struct {
	ID       string   `json:"id"`
	Slug     string   `json:"slug"`
	Tags     []string `json:"tags"`
	MaxRank  *int     `json:"maxRank"`
	Subtypes []string `json:"subtypes"`
}

// Catalog keeps the raw array and parsed entries. Clients receive it unparsed.
type Catalog struct {
	Items    []Item
	Document json.RawMessage
}

type Order struct {
	Type     string  `json:"type"`
	Platinum int     `json:"platinum"`
	Rank     *int    `json:"rank"`
	Subtype  *string `json:"subtype"`
	Visible  bool    `json:"visible"`
	User     struct {
		Status string `json:"status"`
	} `json:"user"`
}

type ClosedEntry struct {
	Datetime string  `json:"datetime"`
	Volume   int     `json:"volume"`
	MinPrice float64 `json:"min_price"`
	MaxPrice float64 `json:"max_price"`
	WAPrice  float64 `json:"wa_price"`
	Median   float64 `json:"median"`
	ModRank  *int    `json:"mod_rank"`
	Subtype  *string `json:"subtype"`
}

type ClosedStatistics struct {
	Last48Hours []ClosedEntry `json:"48hours"`
	Last90Days  []ClosedEntry `json:"90days"`
}

type RivenWeapon struct {
	Slug           string  `json:"slug"`
	GameRef        string  `json:"gameRef"`
	Group          string  `json:"group"`
	RivenType      string  `json:"rivenType"`
	Disposition    float64 `json:"disposition"`
	ReqMasteryRank int     `json:"reqMasteryRank"`
	I18n           I18n    `json:"i18n"`
}

type RivenAttribute struct {
	Slug    string  `json:"slug"`
	GameRef string  `json:"gameRef"`
	Prefix  string  `json:"prefix"`
	Suffix  string  `json:"suffix"`
	Unit    *string `json:"unit"`
	I18n    I18n    `json:"i18n"`
}

type I18n struct {
	En struct {
		Name string `json:"name"`
	} `json:"en"`
}

type Auction struct {
	ID            string `json:"id"`
	BuyoutPrice   *int   `json:"buyout_price"`
	StartingPrice int    `json:"starting_price"`
	IsDirectSell  bool   `json:"is_direct_sell"`
	Visible       bool   `json:"visible"`
	Closed        bool   `json:"closed"`
	Private       bool   `json:"private"`
	Updated       string `json:"updated"`
	Item          struct {
		Name         string             `json:"name"`
		Polarity     string             `json:"polarity"`
		ModRank      int                `json:"mod_rank"`
		ReRolls      int                `json:"re_rolls"`
		MasteryLevel int                `json:"mastery_level"`
		Attributes   []AuctionAttribute `json:"attributes"`
	} `json:"item"`
	Owner struct {
		IngameName string `json:"ingame_name"`
		Status     string `json:"status"`
	} `json:"owner"`
}

type AuctionAttribute struct {
	URLName  string  `json:"url_name"`
	Value    float64 `json:"value"`
	Positive bool    `json:"positive"`
}

type AuctionSearch struct {
	Weapon     string
	Polarity   string
	ReRollsMin *int
	ReRollsMax *int
}
