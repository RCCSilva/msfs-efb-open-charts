package sources

import (
	"strings"
	"testing"
	"time"
)

// Texts as PDFium returns them for charts of the UK AIP (shortened).
const (
	ukADC   = "R\r\n09\r\n27\r\nAERO INFO DATE 27 JAN 26 AD 2-EGLL-2-1\r\nEGLL\r\nAERODROME\r\nCHART - ICAO\r\nARP 512839N 0002741W AD ELEV 83FT LONDON HEATHROW\r\nCHANGE (4/26): FIRE FREQ. OBSTACLE."
	ukGMC   = "Hot Spot\r\nHS5\r\n AD 2-EGLL-2-2\r\nLONDON HEATHROW\r\nEGLL\r\nAIRCRAFT GROUND MOVEMENT/ALL TAXIWAYS\r\nCHART - ICAO\r\nARP 512839N 0002741W AD ELEV 83FT\r\nCHANGE (13/25): MAG INFO REVISED."
	ukSID   = "001 00W 000 30W\r\n5130N\r\nSTANDARD DEPARTURE CHART -\r\nINSTRUMENT (SID) - ICAO\r\nRWY 09R/L RWY 27R/L\r\nCPT 5J 4K 3F 3G\r\nAD 2-EGLL-6-1\r\n"
	ukSTAR  = "327°\r\n147°\r\nSTANDARD ARRIVAL CHART -\r\nINSTRUMENT (STAR) - ICAO\r\nCHANGE (13/25): MAG INFO REVISED.\r\nAD 2-EGLL-7-1"
	ukCites = "see AD 2-EGLL-2-4 and AD 2-EGLL-3-1\r\nAERO INFO DATE 27 JAN 26 AD 2-EGLL-2-1\r\nEGLL\r\nAERODROME\r\nCHART - ICAO\r\nARP 5128N"
	ukOld   = "100 0 100 200 300 400 500m\r\nLONDON HEATHROW - HELICOPTER CROSSING OPERATIONS\r\nAERO INFO DATE 10 FEB 14"
)

func TestUKReadsTheReferenceOfTheChart(t *testing.T) {
	var u uk
	cases := []struct {
		name, text, icao, typ, titleHas string
	}{
		{"adc", ukADC, "EGLL", "ADC", "AERODROME CHART"},
		{"gmc", ukGMC, "EGLL", "GMC", "GROUND MOVEMENT"},
		{"sid", ukSID, "EGLL", "SID", "RWY 09R/L"},
		{"star", ukSTAR, "EGLL", "STAR", "(7-1)"},
	}
	for _, c := range cases {
		in, ok := u.RecognizeContent(c.text)
		if !ok || in.ICAO != c.icao || in.Type != c.typ {
			t.Fatalf("%s: %+v ok=%v", c.name, in, ok)
		}
		if !strings.Contains(strings.ToUpper(in.Title), strings.ToUpper(c.titleHas)) {
			t.Fatalf("%s: title %q does not have %q", c.name, in.Title, c.titleHas)
		}
	}
	in, _ := u.RecognizeContent(ukCites)
	if !strings.HasSuffix(in.Title, "(2-1)") {
		t.Fatalf("the chart's own reference is the one after the date, not the ones it cites: %q", in.Title)
	}
	if _, ok := u.RecognizeContent(ukOld); ok {
		t.Fatal("a chart without the reference is not recognised")
	}
}

func TestRegistryReadsContentOfAnyFile(t *testing.T) {
	info, by, ok := RecognizeContent("manual", ukSID)
	if !ok || by != "nats-uk" || info.ICAO != "EGLL" {
		t.Fatalf("%+v %s %v", info, by, ok)
	}
	if _, _, ok := RecognizeContent("manual", "nothing here"); ok {
		t.Fatal("plain text is not a chart reference")
	}
}

func TestUKLocatesTheCycle(t *testing.T) {
	locs := (uk{}).Locate(time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC))
	if len(locs) != 2 || !strings.Contains(locs[0].URL, "2026-10-01-AIRAC") || !strings.Contains(locs[1].URL, "2026-10-29-AIRAC") {
		t.Fatalf("%+v", locs)
	}
	in := (uk{}).Info()
	if in.Country != "GB" || in.ID != "nats-uk" {
		t.Fatalf("%+v", in)
	}
}
