package naming

import (
	"fmt"
	"strconv"
	"time"
)

// The AIRAC cycle (ICAO Annex 15) is the international calendar on which aeronautical information
// changes: a new cycle takes effect every 28 days, on the same dates in every country, and the
// publications of every provider (an AIP, DECEA, the FAA...) are issued for a cycle. It is named by
// the year and the ordinal of the cycle within the year: 2610 is the tenth cycle of 2026.
//
// This is the single place where that calendar lives. Not every chart has a cycle (some are
// issued on other dates), so a cycle is only ever derived from a date that is a cycle's first day.

// airacAnchor is the effective date of a known cycle (2610). The others follow every 28 days.
var airacAnchor = time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)

const airacDays = 28

// AIRAC returns the effective date of the cycle in force on t and its identifier ("2610").
func AIRAC(t time.Time) (time.Time, string) {
	t = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	days := int(t.Sub(airacAnchor).Hours() / 24)
	n := days / airacDays
	if days < 0 && days%airacDays != 0 {
		n--
	}
	eff := airacAnchor.AddDate(0, 0, airacDays*n)
	first := eff // the first effective date of eff's year
	for first.AddDate(0, 0, -airacDays).Year() == eff.Year() {
		first = first.AddDate(0, 0, -airacDays)
	}
	ord := int(eff.Sub(first).Hours()/24)/airacDays + 1
	return eff, fmt.Sprintf("%02d%02d", eff.Year()%100, ord)
}

// NextAIRAC returns the effective date and identifier of the cycle after the one in force on t.
func NextAIRAC(t time.Time) (time.Time, string) {
	eff, _ := AIRAC(t)
	return AIRAC(eff.AddDate(0, 0, airacDays))
}

// CycleOfDate returns the cycle that starts on date ("2026-08-06" gives "2608"), or "" when the
// date is not the first day of a cycle.
func CycleOfDate(date string) string {
	t, err := time.Parse("2006-01-02", date)
	if err != nil {
		return ""
	}
	eff, id := AIRAC(t)
	if !eff.Equal(t) {
		return ""
	}
	return id
}

// DateOfCycle returns the effective date ("2026-08-06") of a cycle identifier ("2608").
func DateOfCycle(cycle string) (string, bool) {
	if len(cycle) != 4 {
		return "", false
	}
	yy, err1 := strconv.Atoi(cycle[:2])
	ord, err2 := strconv.Atoi(cycle[2:])
	if err1 != nil || err2 != nil || ord < 1 || ord > 14 {
		return "", false
	}
	year := 2000 + yy
	// the first cycle of the year is the first aligned date on or after January 1st
	eff, _ := AIRAC(time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC))
	if eff.Year() < year {
		eff = eff.AddDate(0, 0, airacDays)
	}
	eff = eff.AddDate(0, 0, airacDays*(ord-1))
	if eff.Year() != year {
		return "", false
	}
	return eff.Format("2006-01-02"), true
}
