// Package sources describes where charts come from and what each source can do.
//
// A source is a small module with a base contract (Source: describe yourself and recognise the
// names of your files) plus optional abilities, declared by implementing an interface:
//
//	Cataloger  a static list of items the user can open and download
//	Locator    dynamic addresses computed from a date (AIRAC cycle, package of the day...)
//	Packager   understands a downloaded archive and lists the charts inside
//
// A source may have only some of them: a source with just a static list does not need a
// Locator. Upload of a single PDF is not an ability of a source: it always works, and goes
// through the built-in "manual" source.
//
// A source never downloads anything on its own: it lists what exists, computes where the user
// can find it, and indexes what the user already downloaded. The policy of the provider is part
// of the description.
//
// Today DECEA is compiled in; the contracts here are the line a plugin would cross later.
package sources

import (
	"sort"
	"strings"
	"time"

	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/naming"
)

// Text holds pt/en versions of a string.
type Text struct {
	PT string `json:"pt"`
	EN string `json:"en"`
}

// Policy states what the provider of a source allows.
type Policy struct {
	AutomatedFetch bool   `json:"automatedFetch"` // may the app download on its own?
	UserDownloads  bool   `json:"userDownloads"`  // the user must fetch the files
	Redistribution bool   `json:"redistribution"`
	URL            string `json:"url,omitempty"`
	Notes          Text   `json:"notes"`
}

// Info describes a source.
type Info struct {
	ID           string   `json:"id"`
	Builtin      bool     `json:"builtin"` // part of the app itself, not a provider
	Name         Text     `json:"name"`
	Description  Text     `json:"description"`
	Policy       Policy   `json:"policy"`
	Capabilities []string `json:"capabilities"` // derived: "catalog", "locate", "package"
	Guide        []Text   `json:"guide,omitempty"`
	Homepage     string   `json:"homepage,omitempty"`
	CatalogDate  string   `json:"catalogDate,omitempty"`
	CatalogSize  int      `json:"catalogSize,omitempty"`
}

// Source is the base contract.
type Source interface {
	Info() Info
	// Recognize derives chart metadata from a file name (without extension).
	Recognize(stem string) (naming.Info, bool)
}

// CatalogItem is one downloadable chart listed by a source.
type CatalogItem struct {
	Type      string `json:"type"`
	Number    string `json:"number"`
	Name      string `json:"name"`
	Amendment string `json:"amendment"`
	Published string `json:"published"`
	Effective string `json:"effective"`
	File      string `json:"file"` // file name as published
	URL       string `json:"url"`  // official address, to be opened by the user
	search    string
}

// CatalogFilter narrows a catalog listing.
type CatalogFilter struct {
	Text string
	Type string
}

// Cataloger is implemented by sources with a static list of items.
type Cataloger interface {
	Catalog(f CatalogFilter) []CatalogItem
	CatalogInfo() (date string, size int)
}

// Location is an address computed from a date. The app shows it; the user opens it.
type Location struct {
	Kind      string `json:"kind"` // e.g. "package-full"
	Label     Text   `json:"label"`
	Cycle     string `json:"cycle,omitempty"`     // AIRAC identifier, e.g. "2610"
	Effective string `json:"effective,omitempty"` // YYYY-MM-DD
	Current   bool   `json:"current"`
	URL       string `json:"url"`
	File      string `json:"file,omitempty"` // expected file name
	// Verified is false when the address was derived from a naming pattern and not confirmed.
	Verified bool `json:"verified"`
	Notes    Text `json:"notes,omitempty"`
}

// Locator is implemented by sources whose items are found by date.
type Locator interface {
	Locate(on time.Time) []Location
}

// Packager is implemented by sources that understand a downloaded archive.
type Packager interface {
	OpenPackage(path string) (*PackageInfo, error)
}

// ---------------------------------------------------------------- registry

var registry = []Source{decea{}, manual{}}

// All returns the registered sources, with abilities filled in.
func All() []Info {
	out := make([]Info, 0, len(registry))
	for _, s := range registry {
		out = append(out, describe(s))
	}
	return out
}

// Lookup finds a source by id.
func Lookup(id string) (Source, bool) {
	for _, s := range registry {
		if s.Info().ID == id {
			return s, true
		}
	}
	return nil, false
}

// Get returns the description of a source.
func Get(id string) (Info, bool) {
	s, ok := Lookup(id)
	if !ok {
		return Info{}, false
	}
	return describe(s), true
}

func describe(s Source) Info {
	in := s.Info()
	var caps []string
	if c, ok := s.(Cataloger); ok {
		caps = append(caps, "catalog")
		in.CatalogDate, in.CatalogSize = c.CatalogInfo()
	}
	if _, ok := s.(Locator); ok {
		caps = append(caps, "locate")
	}
	if _, ok := s.(Packager); ok {
		caps = append(caps, "package")
	}
	in.Capabilities = caps
	return in
}

// Catalog lists the items of a source, or nil when it has no static list.
func Catalog(sourceID string, f CatalogFilter) []CatalogItem {
	s, ok := Lookup(sourceID)
	if !ok {
		return nil
	}
	c, ok := s.(Cataloger)
	if !ok {
		return nil
	}
	return c.Catalog(f)
}

// Locate returns the dynamic addresses of a source for a date, or nil.
func Locate(sourceID string, on time.Time) []Location {
	s, ok := Lookup(sourceID)
	if !ok {
		return nil
	}
	l, ok := s.(Locator)
	if !ok {
		return nil
	}
	return l.Locate(on)
}

// PackagerFor returns the archive reader of a source.
func PackagerFor(sourceID string) (Packager, bool) {
	s, ok := Lookup(sourceID)
	if !ok {
		return nil, false
	}
	p, ok := s.(Packager)
	return p, ok
}

// Recognize derives metadata from a file name. The named source goes first; the others are
// tried next, so a PDF handed over by hand is still recognised. It returns the id of the
// source that recognised it ("" when none did and only a tidied title is returned).
func Recognize(sourceID, stem string) (naming.Info, string) {
	if s, ok := Lookup(sourceID); ok {
		if in, ok := s.Recognize(stem); ok {
			return in, sourceID
		}
	}
	for _, s := range registry {
		if s.Info().ID == sourceID {
			continue
		}
		if in, ok := s.Recognize(stem); ok {
			return in, s.Info().ID
		}
	}
	return naming.Info{Title: naming.Tidy(stem)}, ""
}

// ---------------------------------------------------------------- shared helpers

// matchItems applies a text/type filter to catalog items (token prefix, accent-insensitive).
func matchItems(items []CatalogItem, f CatalogFilter) []CatalogItem {
	words := naming.Tokens(f.Text)
	var out []CatalogItem
	for _, it := range items {
		if f.Type != "" && !strings.EqualFold(it.Type, f.Type) {
			continue
		}
		ok := true
		hay := strings.Fields(it.search)
		for _, w := range words {
			found := false
			for _, h := range hay {
				if strings.HasPrefix(h, w) || (len(w) >= 3 && strings.Contains(h, w)) {
					found = true
					break
				}
			}
			if !found {
				ok = false
				break
			}
		}
		if ok {
			out = append(out, it)
		}
	}
	return out
}

func sortItems(items []CatalogItem) {
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Type != items[j].Type {
			return items[i].Type < items[j].Type
		}
		return items[i].Name < items[j].Name
	})
}

// MatchCatalog finds the catalog item whose published file name equals file, so a PDF the
// user downloaded from a provider's page can be named exactly as the provider lists it.
func MatchCatalog(sourceID, file string) (CatalogItem, bool) {
	s, ok := Lookup(sourceID)
	if !ok {
		return CatalogItem{}, false
	}
	c, ok := s.(Cataloger)
	if !ok {
		return CatalogItem{}, false
	}
	want := strings.ToLower(strings.TrimSpace(file))
	for _, it := range c.Catalog(CatalogFilter{}) {
		if strings.ToLower(it.File) == want {
			return it, true
		}
	}
	return CatalogItem{}, false
}
