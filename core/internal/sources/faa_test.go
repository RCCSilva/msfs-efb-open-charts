package sources

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const faaSampleXML = "\xef\xbb\xbf" + `<?xml version="1.0" encoding="utf-8" standalone="yes"?>
<digital_tpp cycle="2610" from_edate="0901Z  10/01/26" to_edate="0901Z  10/29/26">
  <state_code ID="NY" state_fullname="NEW YORK">
    <city_name ID="NEW YORK" volume="NE-2">
      <airport_name ID="JOHN F KENNEDY INTL" military="N" apt_ident="JFK" icao_ident="KJFK" alnum="610">
        <record><chart_code>IAP</chart_code><chart_name>ILS OR LOC RWY 22L</chart_name><pdf_name>00610IL22L.PDF</pdf_name></record>
        <record><chart_code>APD</chart_code><chart_name>AIRPORT DIAGRAM</chart_name><pdf_name>00610AD.PDF</pdf_name></record>
        <record><chart_code>STR</chart_code><chart_name>CAMRN FOUR</chart_name><pdf_name>00610CAMRN.PDF</pdf_name></record>
      </airport_name>
    </city_name>
  </state_code>
  <state_code ID="AK" state_fullname="ALASKA">
    <city_name ID="ADAK ISLAND" volume="AK-1">
      <airport_name ID="ADAK" military="N" apt_ident="ADK" icao_ident="PADK" alnum="1244">
        <record><chart_code>DP</chart_code><chart_name>ADAK ONE</chart_name><pdf_name>01244ADAK1.PDF</pdf_name></record>
      </airport_name>
      <airport_name ID="SMALL FIELD" military="N" apt_ident="0B1" icao_ident="" alnum="9">
        <record><chart_code>IAP</chart_code><chart_name>RNAV (GPS) RWY 5</chart_name><pdf_name>00009R5.PDF</pdf_name></record>
      </airport_name>
    </city_name>
  </state_code>
</digital_tpp>`

func TestFAAMetafileIsParsedAndNamesAreRecognised(t *testing.T) {
	m, err := ParseFAAMetafile(strings.NewReader(faaSampleXML))
	if err != nil {
		t.Fatal(err)
	}
	if m.Cycle != "2610" || m.From != "2026-10-01" || m.To != "2026-10-29" || len(m.Charts) != 5 || m.Airports() != 3 {
		t.Fatalf("metadata = cycle %q %s..%s, %d charts, %d airports", m.Cycle, m.From, m.To, len(m.Charts), m.Airports())
	}
	SetFAAMetadata(m)
	defer SetFAAMetadata(nil)

	in, ok := faa{}.Recognize("00610il22l") // case does not matter, extension is optional
	if !ok || in.ICAO != "KJFK" || in.Type != "IAC" || in.Title != "ILS OR LOC RWY 22L" || in.Validity != "2026-10-01" {
		t.Fatalf("approach = %+v (%v)", in, ok)
	}
	if in, _ := (faa{}).Recognize("00610AD.PDF"); in.Type != "ADC" || in.Title != "AIRPORT DIAGRAM" {
		t.Fatalf("diagram = %+v", in)
	}
	if in, _ := (faa{}).Recognize("00610CAMRN"); in.Type != "STAR" {
		t.Fatalf("star = %+v", in)
	}
	if in, _ := (faa{}).Recognize("01244ADAK1"); in.ICAO != "PADK" || in.Type != "SID" {
		t.Fatalf("alaska = %+v", in)
	}
	// an airport with no ICAO identifier keeps the FAA one
	if in, _ := (faa{}).Recognize("00009R5"); in.ICAO != "0B1" {
		t.Fatalf("no-ICAO airport = %+v", in)
	}
	// through the registry: a file handed over in "My PDFs" is still understood
	id := Identify("manual", "00610IL22L", "00610IL22L.PDF", "")
	if id.ICAO != "KJFK" || id.Title != "ILS OR LOC RWY 22L" || !id.Confident {
		t.Fatalf("identify = %+v", id)
	}
	if _, ok := (faa{}).Recognize("not-in-the-cycle"); ok {
		t.Fatal("a name that is not in the metadata must not be claimed")
	}
	SetFAAMetadata(nil)
	if _, ok := (faa{}).Recognize("00610IL22L"); ok {
		t.Fatal("without metadata nothing can be recognised")
	}
}

func TestFAAMetafileRefusesOtherXML(t *testing.T) {
	if _, err := ParseFAAMetafile(strings.NewReader(`<?xml version="1.0"?><other/>`)); err == nil {
		t.Fatal("an XML that is not a d-TPP metafile must be refused")
	}
}

func TestFAAPackageGroupsByAirportAndNeedsTheMetadata(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "DDTPPA_261001.zip")
	f, _ := os.Create(zipPath)
	zw := zip.NewWriter(f)
	for _, n := range []string{"00610IL22L.PDF", "00610AD.PDF", "01244ADAK1.PDF", "99999UNKNOWN.PDF", "readme.txt"} {
		w, _ := zw.Create(n)
		_, _ = w.Write([]byte("x"))
	}
	_ = zw.Close()
	_ = f.Close()

	SetFAAMetadata(nil)
	if _, err := (faa{}).OpenPackage(zipPath); err != ErrNeedsFAAMetadata {
		t.Fatalf("without metadata: %v", err)
	}
	m, _ := ParseFAAMetafile(strings.NewReader(faaSampleXML))
	SetFAAMetadata(m)
	defer SetFAAMetadata(nil)
	p, err := (faa{}).OpenPackage(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	if p.Charts != 3 || len(p.Airports) != 2 || p.Date != "2026-10-01" {
		t.Fatalf("package = %d charts, %+v, date %q", p.Charts, p.Airports, p.Date)
	}
	if got := p.Select("KJFK", []string{"IAC"}); len(got) != 1 || got[0].File != "00610IL22L.PDF" {
		t.Fatalf("select = %+v", got)
	}
}

func TestFAAMetadataZipIsRecognisedAsSuch(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "DDTPPE_261001.zip")
	f, _ := os.Create(zipPath)
	zw := zip.NewWriter(f)
	w, _ := zw.Create("d-TPP_Metafile.xml")
	_, _ = w.Write([]byte(faaSampleXML))
	_ = zw.Close()
	_ = f.Close()
	if _, err := (faa{}).OpenPackage(zipPath); err != ErrFAAMetadataArchive {
		t.Fatalf("err = %v", err)
	}
	m, err := ReadFAAMetafileFromZip(zipPath)
	if err != nil || m.Cycle != "2610" {
		t.Fatalf("read from zip: %v %+v", err, m)
	}
}

func TestFAACyclesAreEvery28Days(t *testing.T) {
	d := func(s string) time.Time { x, _ := time.Parse("2006-01-02", s); return x }
	for on, want := range map[string]string{"2026-10-01": "2026-10-01", "2026-10-28": "2026-10-01", "2026-10-29": "2026-10-29", "2026-09-10": "2026-09-03"} {
		if got := faaCycleStart(d(on)).Format("2006-01-02"); got != want {
			t.Errorf("cycle of %s = %s, want %s", on, got, want)
		}
	}
}
