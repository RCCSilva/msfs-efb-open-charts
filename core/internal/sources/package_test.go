package sources

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

func writeZip(t *testing.T, members map[string]string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "pacote-full-20261001.zip")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	for name, body := range members {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = w.Write([]byte(body))
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestPackageListsAirportsWhateverTheSeparator(t *testing.T) {
	path := writeZip(t, map[string]string{
		"SBSP/IAC/sbsp_ils-x-rwy-17r_iac_20261001.pdf": "%PDF a",
		"SBSP/ADC/sbsp_sbsp-adc_adc_20261001.pdf":      "%PDF b",
		`SBGR\IAC\sbgr_vor-rwy-09l_iac_20261001.pdf`:   "%PDF c", // written by a Windows tool
		"README.txt": "not a chart",
	})
	p, err := openDeceaPackage(path)
	if err != nil {
		t.Fatal(err)
	}
	if p.Charts != 3 || len(p.Airports) != 2 {
		t.Fatalf("charts=%d airports=%+v", p.Charts, p.Airports)
	}
	if p.Date != "2026-10-01" {
		t.Errorf("date = %q", p.Date)
	}
	sel := p.Select("SBGR", nil)
	if len(sel) != 1 {
		t.Fatalf("select SBGR = %+v", sel)
	}
	b, err := p.Read(sel[0])
	if err != nil || string(b) != "%PDF c" {
		t.Fatalf("read = %q, %v", b, err)
	}
}

func TestNotAPackageIsRefused(t *testing.T) {
	path := writeZip(t, map[string]string{"photo.png": "x"})
	if _, err := openDeceaPackage(path); err != ErrNotPackage {
		t.Fatalf("err = %v, want ErrNotPackage", err)
	}
}
