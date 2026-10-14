// Package lnmpoints reads the CSV that Little Navmap exports for the user points of its "My Data"
// window: one row per point, with or without a header row.
//
//	Type, Name, Ident, Latitude, Longitude, Elevation, Magnetic Declination, Tags, Description,
//	Region, Visible From, Last Edit, Import Filename
//
// The elevation is in feet. Only the columns that Open Charts keeps are read: type, name, ident,
// latitude, longitude, elevation, tags and description.
package lnmpoints

import (
	"bytes"
	"encoding/csv"
	"errors"
	"io"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/navdb"
)

// MaxBytes is the biggest file that is read.
const MaxBytes = 32 << 20

// the columns of an export without a header row
var defaultColumns = map[string]int{
	"type": 0, "name": 1, "ident": 2, "latitude": 3, "longitude": 4, "elevation": 5,
	"tags": 7, "description": 8,
}

// Parse reads the points of an export. Rows that are not points (no number where a coordinate must
// be) are counted in skipped and left out.
func Parse(r io.Reader) (points []navdb.UserPoint, skipped int, err error) {
	raw, err := io.ReadAll(io.LimitReader(r, MaxBytes+1))
	if err != nil {
		return nil, 0, err
	}
	if len(raw) > MaxBytes {
		return nil, 0, errors.New("the file is too big")
	}
	raw = bytes.TrimPrefix(raw, []byte{0xEF, 0xBB, 0xBF})
	if !utf8.Valid(raw) { // an old export in the Windows code page
		runes := make([]rune, len(raw))
		for i, b := range raw {
			runes[i] = rune(b)
		}
		raw = []byte(string(runes))
	}
	cr := csv.NewReader(bytes.NewReader(raw))
	cr.FieldsPerRecord = -1
	cr.LazyQuotes = true
	cr.TrimLeadingSpace = true
	cols := defaultColumns
	first := true
	for {
		rec, err := cr.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			skipped++
			continue
		}
		if first {
			first = false
			if h := headerColumns(rec); h != nil {
				cols = h
				continue
			}
		}
		get := func(name string) string {
			i, ok := cols[name]
			if !ok || i >= len(rec) {
				return ""
			}
			return strings.TrimSpace(rec[i])
		}
		lat, err1 := number(get("latitude"))
		lon, err2 := number(get("longitude"))
		if err1 != nil || err2 != nil || lat < -90 || lat > 90 || lon < -180 || lon > 180 {
			skipped++
			continue
		}
		alt, _ := number(get("elevation"))
		points = append(points, navdb.UserPoint{
			Type: get("type"), Name: get("name"), Ident: get("ident"),
			Lat: lat, Lon: lon, AltFt: alt, Tags: get("tags"), Note: get("description"),
		})
	}
	if len(points) == 0 && skipped == 0 {
		return nil, 0, errors.New("the file has no rows")
	}
	return points, skipped, nil
}

// headerColumns maps column names to positions when the first row is a header, nil otherwise.
func headerColumns(rec []string) map[string]int {
	out := map[string]int{}
	for i, f := range rec {
		name := strings.ToLower(strings.TrimSpace(f))
		switch name {
		case "type", "name", "ident", "latitude", "longitude", "elevation", "tags", "description":
			out[name] = i
		case "lat", "laty":
			out["latitude"] = i
		case "lon", "lonx", "lng":
			out["longitude"] = i
		}
	}
	if _, ok := out["latitude"]; !ok {
		return nil
	}
	if _, ok := out["longitude"]; !ok {
		return nil
	}
	return out
}

func number(s string) (float64, error) {
	return strconv.ParseFloat(strings.ReplaceAll(strings.TrimSpace(s), ",", "."), 64)
}
