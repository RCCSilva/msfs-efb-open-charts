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
