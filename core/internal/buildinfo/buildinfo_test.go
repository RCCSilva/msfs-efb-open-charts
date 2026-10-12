package buildinfo

import (
	"strings"
	"testing"
)

func TestDevelopmentBuildShowsASuffix(t *testing.T) {
	got := Get("0.1.1")
	if got.Release || !strings.HasPrefix(got.Display, "0.1.1-") || got.Version != "0.1.1" {
		t.Fatalf("development build = %+v", got)
	}
}

func TestReleaseBuildShowsTheBareVersion(t *testing.T) {
	Release = "1"
	defer func() { Release = "" }()
	got := Get("0.1.1")
	if !got.Release || got.Display != "0.1.1" {
		t.Fatalf("release build = %+v", got)
	}
}
