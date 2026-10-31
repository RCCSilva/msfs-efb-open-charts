// Package chartfox talks to the ChartFox API (https://chartfox.org/developers), an official provider API the
// project was granted access to. See "Security and policy" in AGENTS.md for the rules that come with it: the user
// connects their own account, every call starts from a user action, answers are cached, the provider is credited and
// the data never leaves the user's data folder.
//
// The types below only hold what the app uses. In particular the chart detail carries no `source_url`: the API hands
// out a link to the original publisher (with that publisher's API key inside it) and the app must never keep or follow it.
package chartfox

// Attribution is the credit ChartFox asks for wherever its data shows (API terms, "Data Copyright").
const (
	Credit    = "Chart data powered by ChartFox"
	CreditURL = "https://chartfox.org"
)

// Airport is an airport of the ChartFox catalogue.
type Airport struct {
	ID         int     `json:"id"`
	Ident      string  `json:"ident"`
	ICAO       string  `json:"icao_code"`
	IATA       string  `json:"iata_code"`
	Name       string  `json:"name"`
	Country    string  `json:"iso_a2_country"`
	Latitude   float64 `json:"latitude"`
	Longitude  float64 `json:"longitude"`
	HasCharts  bool    `json:"has_charts"`
	HasSources bool    `json:"has_sources"`
}

// Chart is one entry of an airport's chart list.
type Chart struct {
	ID          string `json:"id"`
	ParentID    string `json:"parent_id"`
	AirportICAO string `json:"airport_icao"`
	Name        string `json:"name"`
	Code        string `json:"code"`
	TypeKey     string `json:"type_key"`
	Georef      bool   `json:"has_georeferences"`
}

// Detail is the full description of a chart. It has no source URL on purpose (see the package comment).
type Detail struct {
	Chart
	ViewURL string   `json:"view_url"`
	Source  Source   `json:"source"`
	Files   []File   `json:"files"`
	Georefs []GeoRef `json:"georefs"`
}

// Source is the publisher of a chart, whose copyright must show next to it.
type Source struct {
	Name           string `json:"name"`
	DisplayName    string `json:"display_name"`
	CopyrightShort string `json:"copyright_statement_short"`
	CopyrightLong  string `json:"copyright_statement_long"`
}

// File is a copy of the chart hosted by ChartFox.
type File struct {
	Type int    `json:"type"`
	URL  string `json:"url"`
}

// page is the envelope of a paginated list. The documentation shows the paging fields at the top level, the API puts
// them under "meta"; only "meta" is read.
type page[T any] struct {
	Data []T `json:"data"`
	Meta struct {
		CurrentPage int `json:"current_page"`
		LastPage    int `json:"last_page"`
	} `json:"meta"`
}
