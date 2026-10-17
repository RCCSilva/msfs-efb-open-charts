package naming

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPatternReadsTheAttributesOfAName(t *testing.T) {
	cases := []struct {
		pattern, name string
		want          Info
	}{
		{"{icao}_{type}_{title}_{date:YYYYMMDD}", "SBSP_IAC_ILS-X-RWY-17R_20260806.pdf",
			Info{ICAO: "SBSP", Type: "IAC", Title: "ILS X RWY 17R", Validity: "2026-08-06"}},
		{"{icao} - {type} - {title}", "sbgr - sid - rnav evkox 2b", Info{ICAO: "SBGR", Type: "SID", Title: "RNAV EVKOX 2B"}},
		// a separator in the pattern matches any of "_", "-" or a space
		{"{icao}_{type}_{title}", "SBRJ-STAR-NEVOA 1A", Info{ICAO: "SBRJ", Type: "STAR", Title: "NEVOA 1A"}},
		{"{type}_{icao}_{date:DD-MM-YYYY}", "ADC_SBSJ_16-04-2026", Info{ICAO: "SBSJ", Type: "ADC", Validity: "2026-04-16"}},
		{"{icao}_{title}_{date}", "SBSP_VOR Y RWY 35L_2026-08-06", Info{ICAO: "SBSP", Title: "VOR Y RWY 35L", Validity: "2026-08-06"}},
		{"{skip}_{icao}_{type}", "scan0042_SBSP_IAC", Info{ICAO: "SBSP", Type: "IAC"}},
		{"{icao}_{type}_{date:YY-MM-DD}.pdf", "SBSP_IAC_26-08-06", Info{ICAO: "SBSP", Type: "IAC", Validity: "2026-08-06"}},
		// a title that only repeats ICAO and type says nothing
		{"{icao}_{type}_{title}", "SBSP_ADC_SBSP-ADC", Info{ICAO: "SBSP", Type: "ADC"}},
	}
	for _, c := range cases {
		t.Run(c.pattern+" on "+c.name, func(t *testing.T) {
			// Arrange
			p, err := ParsePattern(c.pattern)
			require.NoError(t, err)

			// Act
			got, ok := p.Match(c.name)

			// Assert
			assert.True(t, ok)
			assert.Equal(t, c.want, got)
		})
	}
}

func TestPatternDoesNotMatchOtherNames(t *testing.T) {
	for _, name := range []string{"SBSP_IAC", "SBSPX_IAC_20260806", "chart.pdf", "SBSP_IAC_2026-08-06"} {
		t.Run(name, func(t *testing.T) {
			// Arrange
			p, err := ParsePattern("{icao}_{type}_{date:YYYYMMDD}")
			require.NoError(t, err)

			// Act
			_, ok := p.Match(name)

			// Assert
			assert.False(t, ok)
		})
	}
}

func TestPatternMistakesAreRefused(t *testing.T) {
	for _, bad := range []string{"", "{icao}_{icao}", "{airport}_{type}", "just text", "{date:YYYY}", "{icao:xx}"} {
		t.Run(bad, func(t *testing.T) {
			// Act
			_, err := ParsePattern(bad)

			// Assert
			assert.Error(t, err)
		})
	}
}

func TestPatternReadsAnAIRACCycle(t *testing.T) {
	// Arrange
	p, err := ParsePattern("{icao}_{type}_{title}_{cycle}")
	require.NoError(t, err)

	// Act
	got, ok := p.Match("SBSP_IAC_ILS-X-RWY-17R_2608")

	// Assert
	assert.True(t, ok)
	assert.Equal(t, "2026-08-06", got.Validity)
	assert.Equal(t, "SBSP", got.ICAO)
}

func TestPatternRefusesACycleAndADateTogether(t *testing.T) {
	// Act
	_, err := ParsePattern("{cycle}_{date}")

	// Assert
	assert.Error(t, err, "a chart has one validity")
}
