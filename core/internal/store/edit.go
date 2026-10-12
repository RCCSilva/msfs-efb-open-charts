package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/naming"
)

// Patch lists what a user may change on a chart after it was imported. A nil field is left alone.
// The source, id, image and georeference are never editable. The display name is generated from
// ICAO, type and title, so editing those refreshes it; Name overrides that only when given.
type Patch struct {
	ICAO     *string
	Type     *string
	Title    *string
	Validity *string
	Name     *string
}

// InvalidError is returned for a value that is not acceptable; Field names the property.
type InvalidError struct {
	Field string
	Msg   string
}

func (e InvalidError) Error() string { return fmt.Sprintf("%s: %s", e.Field, e.Msg) }

var (
	icaoRe = regexp.MustCompile(`^[A-Z0-9]{3,4}$`)
	typeRe = regexp.MustCompile(`^[A-Z0-9]{1,12}$`)
)

const maxTitleRunes = 100

// normalise validates a patch and returns the cleaned values for the fields that were given.
func (p Patch) normalise() (Patch, error) {
	var out Patch
	if p.ICAO != nil {
		v := strings.ToUpper(strings.TrimSpace(*p.ICAO))
		if v != "" && !icaoRe.MatchString(v) {
			return out, InvalidError{"icao", "use 3 or 4 letters or digits, or leave it empty"}
		}
		out.ICAO = &v
	}
	if p.Type != nil {
		v := strings.ToUpper(strings.TrimSpace(*p.Type))
		if v != "" && !typeRe.MatchString(v) {
			return out, InvalidError{"type", "use letters and digits only (for example IAC, ADC, REA), up to 12"}
		}
		out.Type = &v
	}
	if p.Title != nil {
		v := strings.Join(strings.Fields(*p.Title), " ")
		if utf8.RuneCountInString(v) > maxTitleRunes {
			return out, InvalidError{"title", fmt.Sprintf("at most %d characters", maxTitleRunes)}
		}
		out.Title = &v
	}
	if p.Validity != nil {
		v := strings.TrimSpace(*p.Validity)
		if v != "" {
			if _, err := time.Parse("2006-01-02", v); err != nil {
				return out, InvalidError{"validity", "use the format YYYY-MM-DD, or leave it empty"}
			}
		}
		out.Validity = &v
	}
	if p.Name != nil {
		v := strings.Join(strings.Fields(*p.Name), " ")
		if v == "" {
			return out, InvalidError{"name", "cannot be empty"}
		}
		out.Name = &v
	}
	return out, nil
}

// Update changes the editable properties of a chart and rewrites its chart.json. When ICAO, type
// or title change, the display name is generated again from them (unless Name is given).
func (l *Library) Update(id string, p Patch) (*Chart, error) {
	p, err := p.normalise()
	if err != nil {
		return nil, err
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	c, ok := l.charts[id]
	if !ok {
		return nil, os.ErrNotExist
	}
	path := filepath.Join(l.ChartDir(id), "chart.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}

	next := *c
	if p.ICAO != nil {
		next.ICAO = *p.ICAO
	}
	if p.Type != nil {
		next.Type = *p.Type
	}
	if p.Title != nil {
		next.Title = *p.Title
	}
	if p.Validity != nil {
		next.Validity = *p.Validity
	}
	switch {
	case p.Name != nil:
		next.Name = *p.Name
	case p.ICAO != nil || p.Type != nil || p.Title != nil:
		next.Name = generatedName(&next)
	}

	m["icao"], m["type"], m["title"], m["validity"], m["name"] = next.ICAO, next.Type, next.Title, next.Validity, next.Name
	out, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return nil, err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, out, 0o644); err != nil {
		return nil, err
	}
	if err := os.Rename(tmp, path); err != nil {
		return nil, err
	}
	c.ICAO, c.Type, c.Title, c.Validity, c.Name = next.ICAO, next.Type, next.Title, next.Validity, next.Name
	c.index()
	return c, nil
}

// generatedName builds the display name from the chart's properties; when they are all empty it
// falls back to the original file name, then the id.
func generatedName(c *Chart) string {
	if n := naming.Display(c.ICAO, c.Type, c.Title); n != "" {
		return n
	}
	if c.OriginalFile != "" {
		return naming.Tidy(c.OriginalFile)
	}
	return c.ID
}
