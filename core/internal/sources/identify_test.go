package sources

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIdentifyTitle(t *testing.T) {
	cases := []struct {
		name      string
		source    string
		file      string
		viewport  string
		wantTitle string
		wantType  string
		matched   string
		confident bool
	}{
		{"catalog beats a generic PDF title (REA XP1)", "decea-br", "ccv-rea-xp1-sao-paulo_rea_20260319.pdf", "BaseMap_REA", "CCV REA XP1-SÃO PAULO", "REA", "catalog", true},
		{"catalog beats a generic PDF title (REA XP2)", "decea-br", "ccv-rea-xp2-sao-paulo_rea_20260319.pdf", "BaseMap_REA", "CCV REA XP2-SÃO PAULO", "REA", "catalog", true},
		{"catalog also wins from My PDFs", "manual", "ccv-rea-xp1-sao-paulo_rea_20260319.pdf", "BaseMap_REA", "CCV REA XP1-SÃO PAULO", "REA", "catalog", true},
		{"ICAO package name keeps its title", "decea-br", "sbsp_ils-x-rwy-17r_iac_20260806.pdf", "BaseMap", "ILS X RWY 17R", "IAC", "", true},
		{"unknown chart, meaningful PDF title", "manual", "meu-mapa.pdf", "Rota Norte", "Rota Norte", "", "", false},
		{"unknown chart, generic PDF title: tidied file name", "manual", "meu-mapa.pdf", "BaseMap_REA", "MEU MAPA", "", "", false},
		{"unknown chart, no PDF title", "manual", "my-chart_v2.pdf", "", "MY CHART V2", "", "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Arrange
			stem := c.file[:len(c.file)-len(".pdf")]

			// Act
			id := Identify(c.source, stem, c.file, c.viewport)

			// Assert
			assert.Equal(t, c.wantTitle, id.Title)
			assert.Equal(t, c.wantType, id.Type)
			assert.Equal(t, c.matched, id.Matched)
			assert.Equal(t, c.confident, id.Confident)
		})
	}
}

// The full package files what is about no aerodrome (the en-route charts) under "sbxx".
func TestAnEnRouteChartOfThePackageHasNoAerodrome(t *testing.T) {
	// Act
	id := Identify("decea-br", "sbxx_enrc-h5_enrc_20261001", "sbxx_enrc-h5_enrc_20261001.pdf", "")

	// Assert
	assert.Empty(t, id.ICAO, "SBXX is not an aerodrome")
	assert.Equal(t, "ENRC", id.Type)
	assert.Equal(t, "H5", id.Title, "the type is not said twice")
	assert.Equal(t, "2026-10-01", id.Validity)
	assert.True(t, id.Confident)
}

func TestAnEnRouteChartFromMyPDFsIsReadTheSameWay(t *testing.T) {
	// Act
	id := Identify("manual", "sbxx_enrc-h5_enrc_20261001", "sbxx_enrc-h5_enrc_20261001.pdf", "")

	// Assert
	assert.Empty(t, id.ICAO)
	assert.Equal(t, "H5", id.Title)
}

func TestAnAerodromeChartHasNoTitleBeyondItsTypeAndAerodrome(t *testing.T) {
	// Act
	id := Identify("decea-br", "sbsp_sbsp-adc_adc_20261001", "sbsp_sbsp-adc_adc_20261001.pdf", "")

	// Assert
	assert.Equal(t, "SBSP", id.ICAO)
	assert.Equal(t, "ADC", id.Type)
	assert.Empty(t, id.Title)
}

func TestTheTitleOfAPackageFileKeepsWhatItsNameSaysWithoutDoubt(t *testing.T) {
	cases := map[string]string{
		"sbsp_rnav-baian-3a-rwy-17r-17l_sid_20261001":             "RNAV BAIAN 3A RWY 17R/17L",
		"sbsp_rnav-anise-1a---ibdal-2a-rwy-35r-35l_star_20260903": "RNAV ANISE 1A - IBDAL 2A RWY 35R/35L",
		"sbsp_ils-x-rwy-17r_iac_20260806":                         "ILS X RWY 17R",
		"sbsp_rwy-17l-17r-35l-35r_vac_20260416":                   "RWY 17L 17R 35L 35R",
		"sbsp_omni-rwy-17l---17r-35l---35r_sid_20260806":          "OMNI RWY 17L 17R 35L 35R",
	}
	for stem, want := range cases {
		t.Run(stem, func(t *testing.T) {
			// Act
			id := Identify("decea-br", stem, stem+".pdf", "")

			// Assert
			assert.Equal(t, want, id.Title)
		})
	}
}
