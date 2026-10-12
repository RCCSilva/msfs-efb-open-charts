package settings

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaults(t *testing.T) {
	s := Load(t.TempDir())
	if s.Port != DefaultPort || s.LAN || len(s.Token) < 20 {
		t.Fatalf("bad defaults: %+v", s)
	}
	if s.ListenAddr() != "127.0.0.1:43210" {
		t.Fatalf("listen addr = %q", s.ListenAddr())
	}
	// the default must not be one of the busy development ports
	for _, busy := range []int{80, 3000, 5000, 8000, 8080, 8888, 9000} {
		if DefaultPort == busy {
			t.Fatalf("default port %d is a commonly used one", busy)
		}
	}
}

func TestRoundTripAndLAN(t *testing.T) {
	dir := t.TempDir()
	in := Settings{Port: 45000, LAN: true, Token: "abc123", Detail: DetailHigh, Basemap: BasemapDark}
	if err := in.Save(dir); err != nil {
		t.Fatal(err)
	}
	out := Load(dir)
	if out != in {
		t.Fatalf("round trip: got %+v, want %+v", out, in)
	}
	if out.ListenAddr() != "0.0.0.0:45000" {
		t.Fatalf("LAN listen addr = %q", out.ListenAddr())
	}
}

func TestCorruptOrInvalidFallsBack(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, FileName), []byte(`{"port": 80, "lan": true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	s := Load(dir)
	if s.Port != DefaultPort {
		t.Fatalf("a privileged port in the file must be ignored, got %d", s.Port)
	}
	if !s.LAN {
		t.Fatal("valid fields must still be read")
	}
	if err := os.WriteFile(filepath.Join(dir, FileName), []byte(`not json`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := Load(dir); got.Port != DefaultPort || got.LAN {
		t.Fatalf("corrupt file must give defaults, got %+v", got)
	}
}

func TestValidatePort(t *testing.T) {
	for _, p := range []int{0, 80, 1023, 65536, -1} {
		if ValidatePort(p) == nil {
			t.Errorf("port %d should be rejected", p)
		}
	}
	for _, p := range []int{1024, 8080, 43210, 65535} {
		if err := ValidatePort(p); err != nil {
			t.Errorf("port %d should be accepted: %v", p, err)
		}
	}
}

func TestTokensAreUnique(t *testing.T) {
	if NewToken() == NewToken() {
		t.Fatal("tokens must be random")
	}
}

func TestDetailDefaultsAndSizes(t *testing.T) {
	if got := Load(t.TempDir()).Detail; got != DetailNormal {
		t.Fatalf("default detail = %q", got)
	}
	if LongEdgePx(DetailNormal) != 3200 || LongEdgePx(DetailHigh) != 4800 || LongEdgePx("whatever") != 3200 {
		t.Fatal("unexpected sizes")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, FileName), []byte(`{"port": 43210, "detail": "ultra"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := Load(dir).Detail; got != DetailNormal {
		t.Fatalf("unknown detail must fall back to normal, got %q", got)
	}
}

func TestBasemapDefaultAndValidation(t *testing.T) {
	if got := Load(t.TempDir()).Basemap; got != BasemapLight {
		t.Fatalf("default basemap = %q, want light", got)
	}
	for _, ok := range []string{BasemapStandard, BasemapLight, BasemapDark, BasemapNone} {
		if !ValidBasemap(ok) {
			t.Errorf("%q should be valid", ok)
		}
	}
	if ValidBasemap("neon") || ValidBasemap("") {
		t.Error("unknown values must be rejected")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, FileName), []byte(`{"port": 43210, "basemap": "neon"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := Load(dir).Basemap; got != BasemapLight {
		t.Fatalf("unknown basemap must fall back to light, got %q", got)
	}
}
