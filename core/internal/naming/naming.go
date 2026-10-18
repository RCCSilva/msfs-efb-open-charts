// Package naming derives ICAO, chart type, readable title and validity from file names, and
// describes chart types (pt/en) so they can be searched by words as well as by code.
package naming

import (
	"strings"
	"time"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

// Info is what could be inferred from a file name.
type Info struct {
	ICAO     string // "SBSP", or ""
	Type     string // "IAC", "SID", ... or ""
	Title    string // "ILS X RWY 17R"
	Validity string // "2026-08-06" (date in the file name), or ""
	// Sure is set by a recognizer whose reading of the name is conclusive by itself (the FAA's
	// raster names, or a d-TPP file found in the cycle's metadata), so no review is needed.
	Sure bool
}

// Naming rules that belong to a provider live in that provider's source (see package sources).
// This package only holds what is common to all of them.

// Display builds "SBSP · IAC · ILS X RWY 17R".
func Display(icao, typ, title string) string {
	parts := make([]string, 0, 3)
	for _, p := range []string{icao, typ, title} {
		if p = strings.TrimSpace(p); p != "" {
			parts = append(parts, p)
		}
	}
	return strings.Join(parts, " · ")
}

// IsGenericTitle reports whether a title embedded in a PDF says nothing about the chart:
// authoring-tool leftovers such as "BaseMap_REA", "Layer 1" or "Untitled".
func IsGenericTitle(s string) bool {
	f := strings.Join(Tokens(s), " ")
	if f == "" {
		return true
	}
	for _, w := range []string{"basemap", "base map", "untitled", "sem titulo", "layer", "viewport", "document", "documento"} {
		if strings.Contains(f, w) {
			return true
		}
	}
	return f == "map" || f == "mapa" || f == "page" || f == "pagina"
}

// Tidy turns "ils-x_rwy-17r" into "ILS X RWY 17R".
func Tidy(s string) string {
	s = strings.NewReplacer("_", " ", "-", " ").Replace(strings.TrimSuffix(s, ".pdf"))
	return strings.ToUpper(strings.Join(strings.Fields(s), " "))
}

// ParseDate converts "20260806" to "2026-08-06" ("" if it is not a date).
func ParseDate(s string) string {
	t, err := time.Parse("20060102", s)
	if err != nil {
		return ""
	}
	return t.Format("2006-01-02")
}

// Fold lower-cases and strips accents: "São Paulo" -> "sao paulo".
func Fold(s string) string {
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	out, _, err := transform.String(t, strings.ToLower(s))
	if err != nil {
		return strings.ToLower(s)
	}
	return out
}

// Tokens splits folded text into searchable words.
func Tokens(s string) []string {
	return strings.FieldsFunc(Fold(s), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
}

// TypeWords gives pt and en words that should find a chart of this type.
func TypeWords(typ string) string {
	switch strings.ToUpper(typ) {
	case "IAC":
		return "aproximacao instrumentos approach instrument ils rnp vor ndb"
	case "SID":
		return "saida padrao departure standard instrument"
	case "STAR":
		return "chegada padrao arrival standard terminal"
	case "ADC":
		return "aerodromo aerodrome airport taxi pista runway"
	case "VAC":
		return "aproximacao visual visual approach"
	case "PDC":
		return "estacionamento parking docking"
	case "AOC":
		return "obstaculos obstacle aerodrome"
	case "AGMC":
		return "movimento solo ground movement"
	case "ATCSMAC":
		return "altitude minima minimum altitude vigilancia surveillance"
	case "ARC":
		return "area carta area chart"
	case "REA":
		return "rotas especiais aeronaves visual special routes"
	case "REH":
		return "rotas especiais helicopteros helicopter routes"
	case "REUL":
		return "rotas especiais ultraleve ultralight"
	case "WAC":
		return "carta aeronautica mundial world aeronautical"
	case "SEC":
		return "seccional sectional vfr carta visual"
	case "TAC":
		return "area terminal terminal area chart vfr"
	case "HEL":
		return "helicoptero helicopter route chart vfr"
	case "ENRC":
		return "carta de rota enroute ifr baixa alta low high altitude"
	case "ENR":
		return "enroute rota ifr baixa alta low high altitude"
	case "ODP":
		return "saida obstaculos obstacle departure procedure"
	case "HOT":
		return "ponto critico hot spot airport movement"
	case "LAH":
		return "pouso espera land and hold short lahso"
	case "MIN":
		return "minimos minimums alternate takeoff"
	}
	return ""
}
