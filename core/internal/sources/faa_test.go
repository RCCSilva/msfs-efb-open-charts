package sources

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

// withFAAMetadata loads the sample metafile for the test and takes it away after.
func withFAAMetadata(t *testing.T) {
	t.Helper()
	m, err := ParseFAAMetafile(strings.NewReader(faaSampleXML))
	require.NoError(t, err)
	SetFAAMetadata(m)
	t.Cleanup(func() { SetFAAMetadata(nil) })
}

func TestFAAMetafileIsParsed(t *testing.T) {
	// Arrange
	r := strings.NewReader(faaSampleXML)

	// Act
	m, err := ParseFAAMetafile(r)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "2610", m.Cycle)
	assert.Equal(t, "2026-10-01", m.From)
	assert.Equal(t, "2026-10-29", m.To)
	assert.Len(t, m.Charts, 5)
	assert.Equal(t, 3, m.Airports())
}

func TestFAAMetafileRefusesOtherXML(t *testing.T) {
	// Arrange
	r := strings.NewReader(`<?xml version="1.0"?><other/>`)

	// Act
	_, err := ParseFAAMetafile(r)

	// Assert
	assert.Error(t, err, "an XML that is not a d-TPP metafile must be refused")
}

func TestFAARecognisesAnApproachWhateverTheCase(t *testing.T) {
	// Arrange
	withFAAMetadata(t)

	// Act
	in, ok := faa{}.Recognize("00610il22l") // the extension is optional

	// Assert
	require.True(t, ok)
	assert.Equal(t, "KJFK", in.ICAO)
	assert.Equal(t, "IAC", in.Type)
	assert.Equal(t, "ILS OR LOC RWY 22L", in.Title)
	assert.Equal(t, "2026-10-01", in.Validity)
}

func TestFAARecognisesTheTypeOfEachChart(t *testing.T) {
	cases := map[string]struct{ icao, typ string }{
		"00610AD.PDF": {"KJFK", "ADC"},
		"00610CAMRN":  {"KJFK", "STAR"},
		"01244ADAK1":  {"PADK", "SID"},
	}
	for name, want := range cases {
		t.Run(name, func(t *testing.T) {
			// Arrange
			withFAAMetadata(t)

			// Act
			in, ok := faa{}.Recognize(name)

			// Assert
			require.True(t, ok)
			assert.Equal(t, want.icao, in.ICAO)
			assert.Equal(t, want.typ, in.Type)
		})
	}
}

func TestFAAAnAirportWithNoICAOKeepsTheFAAIdentifier(t *testing.T) {
	// Arrange
	withFAAMetadata(t)

	// Act
	in, _ := faa{}.Recognize("00009R5")

	// Assert
	assert.Equal(t, "0B1", in.ICAO)
}

func TestFAAIsUsedThroughTheRegistryForMyPDFs(t *testing.T) {
	// Arrange
	withFAAMetadata(t)

	// Act
	id := Identify("manual", "00610IL22L", "00610IL22L.PDF", "")

	// Assert
	assert.Equal(t, "KJFK", id.ICAO)
	assert.Equal(t, "ILS OR LOC RWY 22L", id.Title)
	assert.True(t, id.Confident)
}

func TestFAADoesNotClaimANameThatIsNotInTheMetadata(t *testing.T) {
	// Arrange
	withFAAMetadata(t)

	// Act
	_, ok := faa{}.Recognize("not-in-the-cycle")

	// Assert
	assert.False(t, ok)
}

func TestFAARecognisesNothingWithoutMetadata(t *testing.T) {
	// Arrange
	SetFAAMetadata(nil)

	// Act
	_, ok := faa{}.Recognize("00610IL22L")

	// Assert
	assert.False(t, ok)
}

// faaZip makes a zip file with the named members.
func faaZip(t *testing.T, name string, members map[string]string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	f, err := os.Create(path)
	require.NoError(t, err)
	zw := zip.NewWriter(f)
	for n, body := range members {
		w, err := zw.Create(n)
		require.NoError(t, err)
		_, _ = w.Write([]byte(body))
	}
	require.NoError(t, zw.Close())
	require.NoError(t, f.Close())
	return path
}

func faaChartsZip(t *testing.T) string {
	t.Helper()
	return faaZip(t, "DDTPPA_261001.zip", map[string]string{
		"00610IL22L.PDF": "x", "00610AD.PDF": "x", "01244ADAK1.PDF": "x", "99999UNKNOWN.PDF": "x", "readme.txt": "x",
	})
}

func TestFAAPackageNeedsTheMetadata(t *testing.T) {
	// Arrange
	SetFAAMetadata(nil)
	zipPath := faaChartsZip(t)

	// Act
	_, err := faa{}.OpenPackage(zipPath)

	// Assert
	assert.Equal(t, ErrNeedsFAAMetadata, err)
}

func TestFAAPackageGroupsTheKnownChartsByAirport(t *testing.T) {
	// Arrange
	withFAAMetadata(t)
	zipPath := faaChartsZip(t)

	// Act
	p, err := faa{}.OpenPackage(zipPath)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, 3, p.Charts)
	assert.Len(t, p.Airports, 2)
	assert.Equal(t, "2026-10-01", p.Date)
}

func TestFAAPackageSelectsByAirportAndType(t *testing.T) {
	// Arrange
	withFAAMetadata(t)
	p, err := faa{}.OpenPackage(faaChartsZip(t))
	require.NoError(t, err)

	// Act
	got := p.Select("KJFK", []string{"IAC"})

	// Assert
	require.Len(t, got, 1)
	assert.Equal(t, "00610IL22L.PDF", got[0].File)
}

func TestFAAMetadataZipIsRecognisedAsSuch(t *testing.T) {
	// Arrange
	zipPath := faaZip(t, "DDTPPE_261001.zip", map[string]string{"d-TPP_Metafile.xml": faaSampleXML})

	// Act
	_, openErr := faa{}.OpenPackage(zipPath)
	m, readErr := ReadFAAMetafileFromZip(zipPath)

	// Assert
	assert.Equal(t, ErrFAAMetadataArchive, openErr)
	require.NoError(t, readErr)
	assert.Equal(t, "2610", m.Cycle)
}
