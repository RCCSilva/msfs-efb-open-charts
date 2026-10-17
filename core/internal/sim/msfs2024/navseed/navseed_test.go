package navseed

import "testing"

func TestUnpack38(t *testing.T) {
	// "BOBIC": B=13 O=26 B=13 I=20 C=14 in base 38 (space 0, ? 1, digits 2-11, letters 12-37)
	v := uint64(0)
	for _, c := range "BOBIC" {
		v = v*38 + uint64(indexOf(byte(c)))
	}
	if got := unpack38(v); got != "BOBIC" {
		t.Fatalf("got %q", got)
	}
}

func indexOf(c byte) int {
	for i := 0; i < len(base38); i++ {
		if base38[i] == c {
			return i
		}
	}
	return -1
}

// With the simulator installed this reads the real files; elsewhere it is skipped.
func TestReadReal(t *testing.T) {
	paths := Locate()
	if len(paths) == 0 {
		t.Skip("no simulator packages on this computer")
	}
	list, err := Read(paths, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%d archives, %d identifiers", len(paths), len(list))
	if len(list) < 1000 {
		t.Fatalf("only %d identifiers", len(list))
	}
	found := false
	for _, s := range list {
		if s.Ident == "BOBIC" {
			found = true
			t.Logf("BOBIC at %.3f %.3f", s.Lat, s.Lon)
		}
	}
	if !found {
		t.Log("BOBIC not among the identifiers (not an error: the file may not hold it)")
	}
}
