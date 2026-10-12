package naming

import (
	"testing"
	"time"
)

func TestCycleOfAnEffectiveDate(t *testing.T) {
	// dates of published charts, from three countries' calendars: all the same AIRAC grid
	cases := map[string]string{
		"2026-01-22": "2601", "2026-08-06": "2608", "2026-10-01": "2610", "2026-10-29": "2611",
		"2026-12-24": "2613", "2025-08-07": "2508", "2024-11-28": "2412", "2024-12-26": "2413",
		"2027-01-21": "2701",
		// not the first day of a cycle: no cycle
		"2026-05-20": "", "2026-10-02": "", "nonsense": "",
	}
	for date, want := range cases {
		if got := CycleOfDate(date); got != want {
			t.Errorf("CycleOfDate(%s) = %q, want %q", date, got, want)
		}
	}
}

func TestDateOfCycleIsTheInverse(t *testing.T) {
	for _, c := range []string{"2601", "2608", "2610", "2613", "2508", "2412", "2413", "2701"} {
		d, ok := DateOfCycle(c)
		if !ok || CycleOfDate(d) != c {
			t.Errorf("DateOfCycle(%s) = %s (%v) does not come back", c, d, ok)
		}
	}
	for _, bad := range []string{"", "26", "2600", "2699", "abcd", "2614x"} {
		if d, ok := DateOfCycle(bad); ok {
			t.Errorf("DateOfCycle(%q) = %s, want refused", bad, d)
		}
	}
}

func TestAIRACInForceAndNext(t *testing.T) {
	on := time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC)
	if eff, id := AIRAC(on); id != "2610" || eff.Format("2006-01-02") != "2026-10-01" {
		t.Fatalf("in force = %s %s", eff, id)
	}
	if eff, id := NextAIRAC(on); id != "2611" || eff.Format("2006-01-02") != "2026-10-29" {
		t.Fatalf("next = %s %s", eff, id)
	}
}
