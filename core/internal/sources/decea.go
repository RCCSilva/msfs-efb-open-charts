package sources

import (
	_ "embed"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"localmovingmap/internal/naming"
)

// DECEA / AISWEB (Brazil). Everything specific to this provider lives in this file and
// package.go: the catalog, the file naming, the dynamic package address and the archive layout.

const (
	deceaPolicyURL = "https://ajuda.decea.mil.br/app/uploads/2019/04/Pol%C3%ADtica-de-Uso-POR.pdf"
	deceaBase      = "https://aisweb.decea.mil.br/cartas/visuais/"
	deceaHub       = "https://aisweb.decea.mil.br/?i=cartas&p=visuais"
	deceaPackage   = "https://static-aisweb.decea.mil.br/cartas/pacote/pacote-full-%s.zip"
	deceaCatalogAt = "2026-10-11"
)

//go:embed data/decea-visuais.psv
var deceaVisuais string

type decea struct{}

func (decea) Info() Info {
	return Info{
		ID:       "decea-br",
		Homepage: deceaHub,
		Name:     Text{PT: "DECEA / AISWEB (Brasil)", EN: "DECEA / AISWEB (Brazil)"},
		Description: Text{
			PT: "Cartas aeronáuticas oficiais do Brasil. O aplicativo guia você até os arquivos; o download é feito por você.",
			EN: "Official Brazilian aeronautical charts. The app guides you to the files; you do the download.",
		},
		Policy: Policy{
			AutomatedFetch: false,
			UserDownloads:  true,
			Redistribution: false,
			URL:            deceaPolicyURL,
			Notes: Text{
				PT: "A política de uso do AISWEB proíbe coleta automatizada e redistribuição. O aplicativo não baixa nada sozinho.",
				EN: "AISWEB's usage policy forbids automated collection and redistribution. The app downloads nothing by itself.",
			},
		},
		Guide: []Text{
			{PT: "Para cartas visuais (REA, WAC...), use o catálogo: cada item abre a página oficial.", EN: "For visual charts (REA, WAC...), use the catalog: each item opens the official page."},
			{PT: "Para aeródromos (ADC, IAC, SID, STAR...), baixe o pacote completo do ciclo AIRAC no AISWEB.", EN: "For aerodromes (ADC, IAC, SID, STAR...), download the full package of the AIRAC cycle from AISWEB."},
			{PT: "Informe o arquivo .zip baixado: o aplicativo lista os aeródromos e você escolhe o que importar.", EN: "Point the app to the downloaded .zip: it lists the aerodromes and you choose what to import."},
		},
	}
}

// ---- naming

var (
	// sbsp_ils-x-rwy-17r_iac_20260806.pdf
	deceaPackageName = regexp.MustCompile(`^([a-z]{4})_(.+)_([a-z]+)_(\d{8})$`)
	// ccv-rea-xp2-sao-paulo_rea_20260319.pdf  /  rea_londrina_2015.pdf
	deceaVisualName = regexp.MustCompile(`^(.+)_([a-z]+)_(\d{8})$`)
)

func (decea) Recognize(stem string) (naming.Info, bool) {
	base := strings.TrimSuffix(strings.ToLower(stem), ".pdf")
	if m := deceaPackageName.FindStringSubmatch(base); m != nil {
		in := naming.Info{ICAO: strings.ToUpper(m[1]), Type: strings.ToUpper(m[3]), Validity: naming.ParseDate(m[4]), Title: naming.Tidy(m[2])}
		// "sbsp-adc" repeats ICAO + type: nothing more to say.
		if strings.EqualFold(in.Title, in.ICAO+" "+in.Type) {
			in.Title = ""
		}
		return in, true
	}
	if m := deceaVisualName.FindStringSubmatch(base); m != nil {
		return naming.Info{Type: strings.ToUpper(m[2]), Title: naming.Tidy(m[1]), Validity: naming.ParseDate(m[3])}, true
	}
	return naming.Info{}, false
}

// ---- static catalog (visual charts)

var deceaItems []CatalogItem

func (decea) CatalogInfo() (string, int) { return deceaCatalogAt, len(deceaCatalog()) }

func (decea) Catalog(f CatalogFilter) []CatalogItem { return matchItems(deceaCatalog(), f) }

func deceaCatalog() []CatalogItem {
	if deceaItems != nil {
		return deceaItems
	}
	lines := strings.Split(strings.ReplaceAll(deceaVisuais, "\r\n", "\n"), "\n")
	items := make([]CatalogItem, 0, len(lines))
	for i, ln := range lines {
		if i == 0 || strings.TrimSpace(ln) == "" {
			continue
		}
		p := strings.Split(ln, "|")
		if len(p) < 7 {
			continue
		}
		path := p[6]
		it := CatalogItem{
			Type: p[0], Number: p[1], Name: p[2], Amendment: p[3],
			Published: dmy(p[4]), Effective: dmy(p[5]),
			File: path[strings.LastIndex(path, "/")+1:],
			URL:  deceaBase + escapePath(path),
		}
		it.search = naming.Fold(strings.Join([]string{it.Type, it.Number, it.Name, it.File, naming.TypeWords(it.Type)}, " "))
		items = append(items, it)
	}
	sortItems(items)
	deceaItems = items
	return items
}

// escapePath percent-encodes each path segment ("CNAV 6231 - GEOPDF.pdf" has spaces).
func escapePath(p string) string {
	parts := strings.Split(p, "/")
	for i, s := range parts {
		parts[i] = url.PathEscape(s)
	}
	return strings.Join(parts, "/")
}

// dmy converts 14/10/2010 to 2010-10-14.
func dmy(s string) string {
	p := strings.Split(strings.TrimSpace(s), "/")
	if len(p) != 3 {
		return s
	}
	return p[2] + "-" + p[1] + "-" + p[0]
}

// ---- dynamic: the package of an AIRAC cycle

// airacAnchor is a known effective date (AIRAC 2610). Cycles repeat every 28 days.
var airacAnchor = time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)

// AIRACCycle returns the effective date of the cycle that contains t, and its identifier
// ("2610": year + ordinal of the cycle within the year).
func AIRACCycle(t time.Time) (time.Time, string) {
	t = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	days := int(t.Sub(airacAnchor).Hours() / 24)
	n := days / 28
	if days < 0 && days%28 != 0 {
		n--
	}
	eff := airacAnchor.AddDate(0, 0, 28*n)
	// ordinal: step back to the first effective date of eff's year
	first := eff
	for first.AddDate(0, 0, -28).Year() == eff.Year() {
		first = first.AddDate(0, 0, -28)
	}
	ord := int(eff.Sub(first).Hours()/24)/28 + 1
	return eff, fmt.Sprintf("%02d%02d", eff.Year()%100, ord)
}

// Locate gives candidate addresses of the full package. The file name carries a date
// (pacote-full-YYYYMMDD.zip) whose rule is not documented: a package named after the day it was
// generated has been seen, so both the given day and the AIRAC cycle dates are offered.
// Every address is marked unverified; the official page is the fallback.
func (decea) Locate(on time.Time) []Location {
	notes := Text{
		PT: "Endereço derivado do padrão de nome do pacote; se não abrir, use a página oficial do AISWEB.",
		EN: "Address derived from the package naming pattern; if it does not open, use the official AISWEB page.",
	}
	seen := map[string]bool{}
	var out []Location
	add := func(day time.Time, label Text, cycle string, current bool) {
		d := day.Format("20060102")
		if seen[d] {
			return
		}
		seen[d] = true
		out = append(out, Location{
			Kind: "package-full", Label: label, Cycle: cycle, Effective: day.Format("2006-01-02"),
			Current: current, URL: fmt.Sprintf(deceaPackage, d), File: "pacote-full-" + d + ".zip", Notes: notes,
		})
	}
	day := time.Date(on.Year(), on.Month(), on.Day(), 0, 0, 0, 0, time.UTC)
	cur, id := AIRACCycle(on)
	add(day, Text{PT: "Pacote completo de " + day.Format("02/01/2006"), EN: "Full package of " + day.Format("2006-01-02")}, id, true)
	add(cur, Text{PT: "Pacote completo do ciclo " + id, EN: "Full package of cycle " + id}, id, false)
	next := cur.AddDate(0, 0, 28)
	_, nid := AIRACCycle(next)
	add(next, Text{PT: "Pacote completo do próximo ciclo " + nid, EN: "Full package of next cycle " + nid}, nid, false)
	return out
}

// OpenPackage indexes a DECEA package zip.
func (decea) OpenPackage(path string) (*PackageInfo, error) { return openDeceaPackage(path) }
