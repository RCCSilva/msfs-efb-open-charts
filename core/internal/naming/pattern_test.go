package naming

import "testing"

func TestPatternReadsTheAttributesOfAName(t *testing.T) {
	cases := []struct {
		pattern, name string
		want          Info
	}{
		{"{icao}_{type}_{title}_{date:YYYYMMDD}", "SBSP_IAC_ILS-X-RWY-17R_20260806.pdf",
			Info{ICAO: "SBSP", Type: "IAC", Title: "ILS X RWY 17R", Validity: "2026-08-06"}},
		{"{icao} - {type} - {title}", "sbgr - sid - rnav evkox 2b", Info{ICAO: "SBGR", Type: "SID", Title: "RNAV EVKOX 2B"}},
		// a separator in the pattern matches any of "_", "-" or a space
		{"{icao}_{type}_{title}", "SBRJ-STAR-NEVOA 1A", Info{ICAO: "SBRJ", Type: "STAR", Title: "NEVOA 1A"}},
		{"{type}_{icao}_{date:DD-MM-YYYY}", "ADC_SBSJ_16-04-2026", Info{ICAO: "SBSJ", Type: "ADC", Validity: "2026-04-16"}},
		{"{icao}_{title}_{date}", "SBSP_VOR Y RWY 35L_2026-08-06", Info{ICAO: "SBSP", Title: "VOR Y RWY 35L", Validity: "2026-08-06"}},
		{"{skip}_{icao}_{type}", "scan0042_SBSP_IAC", Info{ICAO: "SBSP", Type: "IAC"}},
		{"{icao}_{type}_{date:YY-MM-DD}.pdf", "SBSP_IAC_26-08-06", Info{ICAO: "SBSP", Type: "IAC", Validity: "2026-08-06"}},
		// a title that only repeats ICAO and type says nothing
		{"{icao}_{type}_{title}", "SBSP_ADC_SBSP-ADC", Info{ICAO: "SBSP", Type: "ADC"}},
	}
	for _, c := range cases {
		p, err := ParsePattern(c.pattern)
		if err != nil {
			t.Fatalf("%q: %v", c.pattern, err)
		}
		got, ok := p.Match(c.name)
		if !ok || got != c.want {
			t.Errorf("%q on %q = %+v (%v), want %+v", c.pattern, c.name, got, ok, c.want)
		}
	}
}

func TestPatternDoesNotMatchOtherNames(t *testing.T) {
	p, err := ParsePattern("{icao}_{type}_{date:YYYYMMDD}")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"SBSP_IAC", "SBSPX_IAC_20260806", "chart.pdf", "SBSP_IAC_2026-08-06"} {
		if _, ok := p.Match(name); ok {
			t.Errorf("%q should not match", name)
		}
	}
}

func TestPatternMistakesAreExplained(t *testing.T) {
	for _, bad := range []string{"", "{icao}_{icao}", "{airport}_{type}", "just text", "{date:YYYY}", "{icao:xx}"} {
		if _, err := ParsePattern(bad); err == nil {
			t.Errorf("%q should be refused", bad)
		}
	}
}

func TestPatternReadsAnAIRACCycle(t *testing.T) {
	p, err := ParsePattern("{icao}_{type}_{title}_{cycle}")
	if err != nil {
		t.Fatal(err)
	}
	got, ok := p.Match("SBSP_IAC_ILS-X-RWY-17R_2608")
	if !ok || got.Validity != "2026-08-06" || got.ICAO != "SBSP" {
		t.Fatalf("got %+v (%v)", got, ok)
	}
	if _, err := ParsePattern("{cycle}_{date}"); err == nil {
		t.Fatal("a chart has one validity: a cycle and a date together must be refused")
	}
}
