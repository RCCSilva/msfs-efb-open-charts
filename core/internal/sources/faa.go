package sources

import (
	"archive/zip"
	"bufio"
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/naming"
)

// FAA (United States): the Digital Terminal Procedures Publication (d-TPP).
//
// The FAA publishes every instrument approach, departure, arrival and airport diagram as a
// separate PDF, named by an internal number (00610IL22L.PDF), not by airport. The meaning of those
// names is in a metadata file of the same cycle (d-TPP_Metafile.xml, inside the "E" zip of the
// complete set): airport, ICAO identifier, kind of chart and chart name. The user downloads that
// file once per cycle and gives it to the app; with it, every PDF of the cycle is recognised.

const (
	faaPage   = "https://www.faa.gov/air_traffic/flight_info/aeronav/digital_products/dtpp/"
	faaSearch = "https://www.faa.gov/air_traffic/flight_info/aeronav/digital_products/dtpp/search/"
	faaSetURL = "https://aeronav.faa.gov/upload_313-d/terminal/DDTPP%s_%s.zip"
)

// FAAChart is what the metadata says about one PDF.
type FAAChart struct {
	ICAO    string `json:"icao"`    // icao_ident, or the FAA identifier when the airport has no ICAO one
	Ident   string `json:"ident"`   // FAA identifier ("JFK")
	Airport string `json:"airport"` // "JOHN F KENNEDY INTL"
	City    string `json:"city"`
	State   string `json:"state"`
	Code    string `json:"code"` // IAP, DP, STR, APD, MIN, HOT, LAH, ODP, DAU
	Name    string `json:"name"` // "ILS OR LOC RWY 22L"
}

// FAAMetadata is the parsed d-TPP metafile of one cycle.
type FAAMetadata struct {
	Cycle  string              `json:"cycle"` // "2610"
	From   string              `json:"from"`  // YYYY-MM-DD
	To     string              `json:"to"`
	Charts map[string]FAAChart `json:"charts"` // upper-case PDF name without extension
}

// Airports returns how many distinct airports the metadata describes.
func (m *FAAMetadata) Airports() int {
	seen := map[string]bool{}
	for _, c := range m.Charts {
		seen[c.ICAO] = true
	}
	return len(seen)
}

var faaMeta atomic.Pointer[FAAMetadata]

// SetFAAMetadata makes metadata the one used to recognise FAA files (nil forgets it).
func SetFAAMetadata(m *FAAMetadata) { faaMeta.Store(m) }

// CurrentFAAMetadata returns the metadata in use, or nil.
func CurrentFAAMetadata() *FAAMetadata { return faaMeta.Load() }

// ErrFAAMetadataArchive is returned by OpenPackage for the "E" zip, which holds the metadata and
// not charts: the caller reads the metadata from it instead.
var ErrFAAMetadataArchive = errors.New("this zip holds the d-TPP metadata, not charts")

// ErrNeedsFAAMetadata is returned when a chart package is opened before the metadata is known.
var ErrNeedsFAAMetadata = errors.New("add the d-TPP metadata (the \"E\" zip or d-TPP_Metafile.xml of the same cycle) first: it tells which airport each file belongs to")

var faaEffective = regexp.MustCompile(`(\d{2})/(\d{2})/(\d{2})`)

// ParseFAAMetafile reads d-TPP_Metafile.xml (about 16 MB) in one streaming pass.
func ParseFAAMetafile(r io.Reader) (*FAAMetadata, error) {
	br := bufio.NewReader(r)
	if b, err := br.Peek(3); err == nil && bytes.Equal(b, []byte{0xEF, 0xBB, 0xBF}) {
		_, _ = br.Discard(3) // the FAA writes a BOM, which encoding/xml refuses
	}
	d := xml.NewDecoder(br)
	m := &FAAMetadata{Charts: map[string]FAAChart{}}
	var state, city string
	var apt FAAChart
	for {
		tok, err := d.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("d-TPP metafile: %w", err)
		}
		se, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		switch se.Name.Local {
		case "digital_tpp":
			for _, a := range se.Attr {
				switch a.Name.Local {
				case "cycle":
					m.Cycle = a.Value
				case "from_edate":
					m.From = faaDate(a.Value)
				case "to_edate":
					m.To = faaDate(a.Value)
				}
			}
		case "state_code":
			state = attr(se, "state_fullname")
		case "city_name":
			city = attr(se, "ID")
		case "airport_name":
			apt = FAAChart{Airport: attr(se, "ID"), Ident: attr(se, "apt_ident"), ICAO: attr(se, "icao_ident"), City: city, State: state}
			if apt.ICAO == "" {
				apt.ICAO = apt.Ident
			}
		case "record":
			var rec struct {
				Code string `xml:"chart_code"`
				Name string `xml:"chart_name"`
				PDF  string `xml:"pdf_name"`
			}
			if err := d.DecodeElement(&rec, &se); err != nil {
				return nil, fmt.Errorf("d-TPP metafile: %w", err)
			}
			key := strings.ToUpper(strings.TrimSuffix(strings.TrimSpace(rec.PDF), filepath.Ext(rec.PDF)))
			if key == "" {
				continue
			}
			c := apt
			c.Code, c.Name = strings.ToUpper(strings.TrimSpace(rec.Code)), strings.TrimSpace(rec.Name)
			m.Charts[key] = c
		}
	}
	if m.Cycle == "" || len(m.Charts) == 0 {
		return nil, errors.New("this does not look like a d-TPP metafile (no cycle or no charts)")
	}
	return m, nil
}

func attr(se xml.StartElement, name string) string {
	for _, a := range se.Attr {
		if a.Name.Local == name {
			return strings.TrimSpace(a.Value)
		}
	}
	return ""
}

// faaDate turns "0901Z  10/01/26" (MM/DD/YY) into 2026-10-01.
func faaDate(s string) string {
	m := faaEffective.FindStringSubmatch(s)
	if m == nil {
		return ""
	}
	return "20" + m[3] + "-" + m[1] + "-" + m[2]
}

// ReadFAAMetafileFromZip finds d-TPP_Metafile.xml in a zip (the "E" set) and parses it.
func ReadFAAMetafileFromZip(zipPath string) (*FAAMetadata, error) {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, fmt.Errorf("open zip: %w", err)
	}
	defer zr.Close()
	for _, f := range zr.File {
		if strings.EqualFold(path.Base(strings.ReplaceAll(f.Name, "\\", "/")), "d-TPP_Metafile.xml") {
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			defer rc.Close()
			return ParseFAAMetafile(rc)
		}
	}
	return nil, errors.New("d-TPP_Metafile.xml is not in this zip")
}

// faaType maps the FAA chart code to the type used across the app.
func faaType(code string) string {
	switch strings.ToUpper(code) {
	case "IAP":
		return "IAC" // instrument approach chart
	case "DP":
		return "SID"
	case "STR":
		return "STAR"
	case "APD":
		return "ADC"
	}
	return strings.ToUpper(code) // ODP, HOT, LAH, MIN, DAU
}

// ---------------------------------------------------------------- the source

type faa struct{}

func (faa) Info() Info {
	return Info{
		ID:            "faa-us",
		Homepage:      faaPage,
		HomepageLabel: Text{PT: "FAA: página dos procedimentos (d-TPP)", EN: "FAA: terminal procedures page (d-TPP)"},
		HomepageNote:  Text{PT: "os pacotes do ciclo (A a D) e os metadados (E) estão nesta página", EN: "the packages of the cycle (A to D) and the metadata (E) are on this page"},
		Name:          Text{PT: "FAA (Estados Unidos)", EN: "FAA (United States)"},
		Description: Text{
			PT: "Procedimentos de terminal (d-TPP) da FAA: aproximações, saídas, chegadas e diagramas de aeroporto.",
			EN: "FAA terminal procedures (d-TPP): approaches, departures, arrivals and airport diagrams.",
		},
		Policy: Policy{
			AutomatedFetch: false,
			UserDownloads:  true,
			Redistribution: false,
			URL:            faaPage,
			Notes: Text{
				PT: "Os arquivos da FAA são públicos, mas o site bloqueia downloads automáticos. Você baixa; o aplicativo interpreta.",
				EN: "FAA files are public, but the site blocks automated downloads. You download; the app reads them.",
			},
		},
		Guide: []Text{
			{PT: "Baixe o arquivo de metadados do ciclo (o zip \"E\", com o d-TPP_Metafile.xml). Ele diz a que aeroporto cada PDF pertence; você faz isso uma vez por ciclo (28 dias).", EN: "Download the metadata of the cycle (the \"E\" zip, with d-TPP_Metafile.xml). It says which airport each PDF belongs to; you do this once per cycle (28 days)."},
			{PT: "Baixe as cartas: pela busca da FAA, aeroporto por aeroporto (PDF), ou os conjuntos completos (zips A a D, cerca de 1 GB cada).", EN: "Download the charts: from the FAA search, airport by airport (PDF), or the complete sets (zips A to D, about 1 GB each)."},
			{PT: "Coloque tudo na área \"Adicione o que você baixou\", logo abaixo. Os PDFs são identificados pelo metadado; de um zip você escolhe os aeroportos.", EN: "Put everything in the \"Add what you downloaded\" area just below. PDFs are recognised through the metadata; from a zip you choose the airports."},
			{PT: "Aproximações e saídas/chegadas vêm georreferenciadas. Os diagramas de aeroporto, não: ficam sem a posição da aeronave.", EN: "Approaches and departures/arrivals are georeferenced. Airport diagrams are not: they have no aircraft position."},
		},
	}
}

var (
	// "Boston HEL", "Boston Downtown HEL", "Albuquerque SEC", "Atlanta TAC", "Caribbean 1 VFR"
	faaVFRName = regexp.MustCompile(`(?i)^(.+?)[ _-]+(SEC|TAC|HEL|GCA|FLY|VFR|WAC)$`)
	// "ENR_L01" (low), "ENR_H01" (high), "ENR_A01" (area), "ENR_AKL01", "ENR_P01"...
	faaEnrouteName = regexp.MustCompile(`(?i)^ENR[_ -]?(AKL|AKH|AKP|L|H|A|P|C)(\d+)$`)
)

// faaRaster reads the name of a VFR or IFR enroute raster (a GeoTIFF), which carries the kind of
// chart in its name; no metadata is needed.
func faaRaster(stem string) (naming.Info, bool) {
	base := strings.TrimSuffix(strings.TrimSpace(stem), filepath.Ext(stem))
	if m := faaEnrouteName.FindStringSubmatch(base); m != nil {
		kind := map[string]string{"L": "LOW ALTITUDE", "H": "HIGH ALTITUDE", "A": "AREA", "P": "PACIFIC", "C": "CARIBBEAN",
			"AKL": "ALASKA LOW", "AKH": "ALASKA HIGH", "AKP": "ALASKA PLANNING"}[strings.ToUpper(m[1])]
		return naming.Info{Type: "ENR", Title: strings.TrimSpace("ENROUTE " + kind + " " + m[2]), Sure: true}, true
	}
	if m := faaVFRName.FindStringSubmatch(base); m != nil {
		return naming.Info{Type: strings.ToUpper(m[2]), Title: naming.Tidy(m[1]), Sure: true}, true
	}
	return naming.Info{}, false
}

// Recognize reads the file name. A d-TPP name is a number, not an airport, so it is read through
// the metadata of the cycle (without it, it cannot be); a VFR or IFR raster carries its kind in
// the name.
func (faa) Recognize(stem string) (naming.Info, bool) {
	if in, ok := faaRaster(stem); ok {
		return in, true
	}
	m := faaMeta.Load()
	if m == nil {
		return naming.Info{}, false
	}
	key := strings.ToUpper(strings.TrimSuffix(strings.TrimSpace(stem), filepath.Ext(stem)))
	c, ok := m.Charts[key]
	if !ok {
		return naming.Info{}, false
	}
	return naming.Info{ICAO: c.ICAO, Type: faaType(c.Code), Title: c.Name, Validity: m.From, Sure: true}, true
}

// Locate gives the addresses of the d-TPP of the cycle in force on a date and of the next one.
// They are the ones listed on the FAA page (verified against it), so they are shown as they are.
func (faa) Locate(on time.Time) []Location {
	// the d-TPP follows the international AIRAC calendar (package naming)
	cur, _ := naming.AIRAC(on)
	var out []Location
	out = append(out, Location{
		Kind: "search", Verified: true, Current: true, URL: faaSearch,
		Label: Text{PT: "Busca de procedimentos da FAA (por aeroporto)", EN: "FAA terminal procedures search (by airport)"},
	})
	next, _ := naming.NextAIRAC(on)
	for i, c := range []time.Time{cur, next} {
		tag := c.Format("060102")
		cycleTxt := Text{PT: "ciclo de " + c.Format("02/01/2006"), EN: "cycle of " + c.Format("2006-01-02")}
		if i == 1 {
			cycleTxt = Text{PT: "próximo ciclo, " + c.Format("02/01/2006"), EN: "next cycle, " + c.Format("2006-01-02")}
		}
		out = append(out, Location{
			Kind: "metadata", Verified: true, Current: i == 0, Effective: c.Format("2006-01-02"),
			URL: fmt.Sprintf(faaSetURL, "E", tag), File: fmt.Sprintf("DDTPPE_%s.zip", tag),
			Label: Text{PT: "Metadados (zip E), " + cycleTxt.PT, EN: "Metadata (zip E), " + cycleTxt.EN},
		})
	}
	return out
}

// OpenPackage indexes one of the complete d-TPP zips (A to D): flat PDFs named by number. The
// airports come from the metadata; nothing is extracted.
func (faa) OpenPackage(zipPath string) (*PackageInfo, error) {
	if !strings.EqualFold(filepath.Ext(zipPath), ".zip") {
		return nil, errors.New("not a .zip file")
	}
	st, err := os.Stat(zipPath)
	if err != nil {
		return nil, err
	}
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, fmt.Errorf("open zip: %w", err)
	}
	defer zr.Close()

	m := faaMeta.Load()
	info := &PackageInfo{Path: zipPath, Size: st.Size(), Modified: st.ModTime()}
	tifs := map[string]*Entry{} // by lower-case stem; a raster has no airport (an empty ICAO)
	sidecars := map[string]string{}
	if mt := regexp.MustCompile(`_(\d{6})`).FindStringSubmatch(filepath.Base(zipPath)); mt != nil {
		if t, err := time.Parse("060102", mt[1]); err == nil {
			info.Date = t.Format("2006-01-02")
		}
	}
	air := map[string]*Airport{}
	for _, f := range zr.File {
		base := path.Base(strings.ReplaceAll(f.Name, "\\", "/"))
		if strings.EqualFold(base, "d-TPP_Metafile.xml") {
			return nil, ErrFAAMetadataArchive
		}
		switch strings.ToLower(path.Ext(base)) {
		case ".tif", ".tiff":
			stem := strings.ToLower(strings.TrimSuffix(base, path.Ext(base)))
			if in, ok := faaRaster(base); ok {
				e := Entry{Member: f.Name, Type: in.Type, File: base, Size: f.UncompressedSize64}
				tifs[stem] = &e
			}
			continue
		case ".htm", ".html":
			sidecars[strings.ToLower(strings.TrimSuffix(base, path.Ext(base)))] = f.Name
			continue
		}
		if !strings.EqualFold(path.Ext(base), ".pdf") {
			continue
		}
		if m == nil {
			return nil, ErrNeedsFAAMetadata
		}
		c, ok := m.Charts[strings.ToUpper(strings.TrimSuffix(base, path.Ext(base)))]
		if !ok {
			continue
		}
		e := Entry{Member: f.Name, ICAO: c.ICAO, Type: faaType(c.Code), File: base, Size: f.UncompressedSize64}
		info.entries = append(info.entries, e)
		a := air[e.ICAO]
		if a == nil {
			a = &Airport{ICAO: e.ICAO, Types: map[string]int{}}
			air[e.ICAO] = a
		}
		a.Count++
		a.Types[e.Type]++
	}
	// VFR and IFR rasters: charts of a region, so they belong to no airport (an empty ICAO)
	stems := make([]string, 0, len(tifs))
	for stem := range tifs {
		stems = append(stems, stem)
	}
	sort.Strings(stems)
	for _, stem := range stems {
		e := *tifs[stem]
		e.Sidecar = sidecars[stem]
		info.entries = append(info.entries, e)
		a := air[""]
		if a == nil {
			a = &Airport{ICAO: "", Types: map[string]int{}}
			air[""] = a
		}
		a.Count++
		a.Types[e.Type]++
	}
	if len(info.entries) == 0 {
		return nil, ErrNotPackage
	}
	info.Charts = len(info.entries)
	for _, a := range air {
		info.Airports = append(info.Airports, *a)
	}
	sort.Slice(info.Airports, func(i, j int) bool { return info.Airports[i].ICAO < info.Airports[j].ICAO })
	return info, nil
}
