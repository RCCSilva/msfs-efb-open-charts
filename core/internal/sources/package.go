package sources

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Entry is one chart inside a DECEA package (ICAO/TYPE/file.pdf).
type Entry struct {
	Member string `json:"member"` // path inside the zip
	ICAO   string `json:"icao"`
	Type   string `json:"type"`
	File   string `json:"file"`
	Size   uint64 `json:"size"`
}

// Airport summarises the charts of one aerodrome in a package.
type Airport struct {
	ICAO  string         `json:"icao"`
	Count int            `json:"count"`
	Types map[string]int `json:"types"`
}

// PackageInfo is what the user sees before choosing what to import.
type PackageInfo struct {
	Path     string    `json:"path"`
	Size     int64     `json:"size"`
	Modified time.Time `json:"modified"`
	Date     string    `json:"date"` // from pacote-full-YYYYMMDD.zip, when present
	Charts   int       `json:"charts"`
	Airports []Airport `json:"airports"`

	entries []Entry
}

// MaxMemberBytes bounds how much of one zip member is read (a chart is a few MB).
const MaxMemberBytes = 64 << 20

var (
	memberRe = regexp.MustCompile(`^([A-Z0-9]{4})/([A-Z0-9]+)/([^/]+\.pdf)$`)
	dateRe   = regexp.MustCompile(`(\d{8})`)
)

// ErrNotPackage is returned when the file is not a DECEA chart package.
var ErrNotPackage = errors.New("this zip does not look like a DECEA chart package (expected ICAO/TYPE/file.pdf)")

// openDeceaPackage indexes a zip without extracting anything. Only the central directory is read.
func openDeceaPackage(path string) (*PackageInfo, error) {
	if !strings.EqualFold(filepath.Ext(path), ".zip") {
		return nil, errors.New("not a .zip file")
	}
	st, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	zr, err := zip.OpenReader(path)
	if err != nil {
		return nil, fmt.Errorf("open zip: %w", err)
	}
	defer zr.Close()

	info := &PackageInfo{Path: path, Size: st.Size(), Modified: st.ModTime()}
	if m := dateRe.FindStringSubmatch(filepath.Base(path)); m != nil {
		if t, err := time.Parse("20060102", m[1]); err == nil {
			info.Date = t.Format("2006-01-02")
		}
	}
	air := map[string]*Airport{}
	for _, f := range zr.File {
		// some Windows tools write the folders with a backslash; the member keeps its real name
		m := memberRe.FindStringSubmatch(strings.ReplaceAll(f.Name, "\\", "/"))
		if m == nil {
			continue
		}
		e := Entry{Member: f.Name, ICAO: m[1], Type: m[2], File: m[3], Size: f.UncompressedSize64}
		info.entries = append(info.entries, e)
		a := air[e.ICAO]
		if a == nil {
			a = &Airport{ICAO: e.ICAO, Types: map[string]int{}}
			air[e.ICAO] = a
		}
		a.Count++
		a.Types[e.Type]++
	}
	if len(info.entries) == 0 {
		return nil, ErrNotPackage
	}
	info.Charts = len(info.entries)
	for _, a := range air {
		info.Airports = append(info.Airports, *a)
	}
	sort.Slice(info.Airports, func(i, j int) bool { return info.Airports[i].ICAO < info.Airports[j].ICAO })
	return info, nil
}

// Entries returns every chart of the package.
func (p *PackageInfo) Entries() []Entry { return append([]Entry(nil), p.entries...) }

// Select returns the entries of the package matching the ICAO and, optionally, the types.
func (p *PackageInfo) Select(icao string, types []string) []Entry {
	want := map[string]bool{}
	for _, t := range types {
		want[strings.ToUpper(t)] = true
	}
	var out []Entry
	for _, e := range p.entries {
		if !strings.EqualFold(e.ICAO, icao) {
			continue
		}
		if len(want) > 0 && !want[e.Type] {
			continue
		}
		out = append(out, e)
	}
	return out
}

// Read returns the bytes of one member. The zip is opened per call; imports are one at a time.
func (p *PackageInfo) Read(e Entry) ([]byte, error) {
	zr, err := zip.OpenReader(p.Path)
	if err != nil {
		return nil, err
	}
	defer zr.Close()
	for _, f := range zr.File {
		if f.Name != e.Member {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		defer rc.Close()
		data, err := io.ReadAll(io.LimitReader(rc, MaxMemberBytes+1))
		if err != nil {
			return nil, err
		}
		if len(data) > MaxMemberBytes {
			return nil, errors.New("member too large")
		}
		return data, nil
	}
	return nil, os.ErrNotExist
}
