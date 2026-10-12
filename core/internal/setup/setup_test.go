package setup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallAndState(t *testing.T) {
	c := filepath.Join(t.TempDir(), "Community")
	if err := os.MkdirAll(c, 0o755); err != nil {
		t.Fatal(err)
	}
	if st := EFBState(c); st.Installed {
		t.Fatal("should not be installed yet")
	}
	if err := InstallEFB(c, "http://127.0.0.1:9000"); err != nil {
		t.Fatal(err)
	}
	st := EFBState(c)
	if !st.Installed || !st.UpToDate || st.Version != EFBVersion {
		t.Fatalf("bad state %+v", st)
	}
	js, _ := os.ReadFile(filepath.Join(c, EFBPackageName, "html_ui/efb_ui/efb_apps/OpenCharts/OpenCharts.js"))
	if !strings.Contains(string(js), "var SERVER_URL = 'http://127.0.0.1:9000/';") {
		t.Fatal("server url not written")
	}
	lay, _ := os.ReadFile(filepath.Join(c, EFBPackageName, "layout.json"))
	if !strings.Contains(string(lay), "html_ui/efb_ui/efb_apps/opencharts/opencharts.js") {
		t.Fatalf("layout paths must be lowercase: %s", lay)
	}
	if err := RemoveEFB(c); err != nil || EFBState(c).Installed {
		t.Fatal("remove failed")
	}
	if err := RemoveEFB(filepath.Join(t.TempDir(), "Other")); err == nil {
		t.Fatal("must refuse a folder that is not Community")
	}
}

func TestAutoStartKeepsOthers(t *testing.T) {
	x := filepath.Join(t.TempDir(), "EXE.xml")
	orig := "<?xml version=\"1.0\" encoding=\"Windows-1252\"?>\r\n<SimBase.Document Type=\"Launch\" version=\"1,0\">\r\n" +
		"\t<Launch.Addon>\r\n\t\t<Name>Other Tool</Name>\r\n\t\t<Path>C:/x/other.exe</Path>\r\n\t</Launch.Addon>\r\n</SimBase.Document>\r\n"
	if err := os.WriteFile(x, []byte(orig), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := SetAutoStart(x, `C:\Apps\opencharts.exe`, []string{"-data", `C:\d`}, true); err != nil {
		t.Fatal(err)
	}
	if !AutoStartEnabled(x) {
		t.Fatal("not enabled")
	}
	b, _ := os.ReadFile(x)
	if !strings.Contains(string(b), "Other Tool") {
		t.Fatal("other addon lost")
	}
	if err := SetAutoStart(x, `C:\Apps\opencharts.exe`, nil, true); err != nil { // idempotent
		t.Fatal(err)
	}
	b, _ = os.ReadFile(x)
	if strings.Count(string(b), "<Name>Open Charts</Name>") != 1 {
		t.Fatal("entry duplicated")
	}
	if err := SetAutoStart(x, "", nil, false); err != nil {
		t.Fatal(err)
	}
	b, _ = os.ReadFile(x)
	if AutoStartEnabled(x) || !strings.Contains(string(b), "Other Tool") {
		t.Fatalf("disable broke the file: %s", b)
	}
	if _, err := os.Stat(x + ".opencharts.bak"); err != nil {
		t.Fatal("backup missing")
	}
	if err := SetAutoStart(x, "C:/Usu\u00e1rio/o.exe", nil, true); err == nil {
		t.Fatal("non-ASCII path must be refused")
	}
}
