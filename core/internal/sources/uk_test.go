package sources

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	cases := []struct {
		name, text, icao, typ, titleHas string
	}{
		{"adc", ukADC, "EGLL", "ADC", "AERODROME CHART"},
		{"gmc", ukGMC, "EGLL", "GMC", "GROUND MOVEMENT"},
		{"sid", ukSID, "EGLL", "SID", "RWY 09R/L"},
		{"star", ukSTAR, "EGLL", "STAR", "(7-1)"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Arrange
			var u uk

			// Act
			in, ok := u.RecognizeContent(c.text)

			// Assert
			require.True(t, ok)
			assert.Equal(t, c.icao, in.ICAO)
			assert.Equal(t, c.typ, in.Type)
			assert.Contains(t, strings.ToUpper(in.Title), strings.ToUpper(c.titleHas))
		})
	}
}

func TestUKTakesTheReferenceAfterTheDateNotTheOnesTheChartCites(t *testing.T) {
	// Arrange
	var u uk

	// Act
	in, _ := u.RecognizeContent(ukCites)

	// Assert
	assert.True(t, strings.HasSuffix(in.Title, "(2-1)"), "title %q", in.Title)
}

func TestUKDoesNotRecogniseAChartWithoutTheReference(t *testing.T) {
	// Arrange
	var u uk

	// Act
	_, ok := u.RecognizeContent(ukOld)

	// Assert
	assert.False(t, ok)
}

func TestRegistryReadsContentOfAnyFile(t *testing.T) {
	// Act
	info, by, ok := RecognizeContent("manual", ukSID)

	// Assert
	require.True(t, ok)
	assert.Equal(t, "nats-uk", by)
	assert.Equal(t, "EGLL", info.ICAO)
}

func TestRegistryDoesNotTakePlainTextForAChartReference(t *testing.T) {
	// Act
	_, _, ok := RecognizeContent("manual", "nothing here")

	// Assert
	assert.False(t, ok)
}

func TestUKLocatesTheCycle(t *testing.T) {
	// Arrange
	on := time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC)

	// Act
	locs := uk{}.Locate(on)

	// Assert
	require.Len(t, locs, 2)
	assert.Contains(t, locs[0].URL, "2026-10-01-AIRAC")
	assert.Contains(t, locs[1].URL, "2026-10-29-AIRAC")
}

func TestUKDescribesItself(t *testing.T) {
	// Act
	in := uk{}.Info()

	// Assert
	assert.Equal(t, "GB", in.Country)
	assert.Equal(t, "nats-uk", in.ID)
}

var ukOn = time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC)

func TestUKPlaceByCodeGoesStraightToTheAerodromePage(t *testing.T) {
	// Act
	places := uk{}.Places("egll", ukOn)

	// Assert
	require.Len(t, places, 1)
	assert.Equal(t, "EGLL", places[0].ICAO)
	assert.True(t, strings.HasSuffix(places[0].URL, "/2026-10-01-AIRAC/html/eAIP/EG-AD-2.EGLL-en-GB.html"), places[0].URL)
}

func TestUKPlaceByName(t *testing.T) {
	// Act
	places := uk{}.Places("heathrow", ukOn)

	// Assert
	require.Len(t, places, 1)
	assert.Equal(t, "EGLL", places[0].ICAO)
}

func TestUKPlaceByANameSeveralShareGivesSeveral(t *testing.T) {
	// Act
	places := uk{}.Places("london", ukOn)

	// Assert
	assert.GreaterOrEqual(t, len(places), 3, "several London aerodromes")
}

func TestUKPlaceThatDoesNotExistGivesNone(t *testing.T) {
	// Act
	places := uk{}.Places("zzzz", ukOn)

	// Assert
	assert.Empty(t, places)
}

func TestASourceWithNoPlacesGivesNone(t *testing.T) {
	// Act
	places := Places("faa-us", "x", ukOn)

	// Assert
	assert.Nil(t, places)
}
