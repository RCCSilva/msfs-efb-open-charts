package naming

import (
	"fmt"
	"regexp"
	"strings"
)

// A Pattern is a file-name template written by the user, such as
//
//	{icao}_{type}_{title}_{date:YYYYMMDD}
//
// so that a whole folder of PDFs named the same way is recognised without answering a question
// for each file. Literal text must match (case does not matter, and "_", "-" and a space are
// interchangeable); a {placeholder} says which attribute that part of the name is.
//
// Placeholders:
//
//	{icao}            the airport code, four letters
//	{type}            the chart type: IAC, SID, STAR, ADC...
//	{title}           free text (the rest of the chart's name)
//	{date}            the date the chart is valid from; {date:FORMAT} fixes the format with the
//	                  tokens YYYY, YY, MM and DD, for example {date:DD-MM-YYYY}. Without a format,
//	                  YYYYMMDD and YYYY-MM-DD are understood. {validity} is the same thing.
//	{cycle}           the AIRAC cycle written as four digits (2610); it stands for the date it takes effect
//	{skip}            any text that is not needed
type Pattern struct {
	text   string
	re     *regexp.Regexp
	fields []string       // capture group names in order: icao, type, title, date
	dates  map[int]string // group index -> date format ("" = automatic)
	// cycleGroups are the capture groups that hold an AIRAC cycle, not a date
	cycleGroups []int
}

// PlaceholderNames are the placeholders a pattern may use, for messages and for the UI.
var PlaceholderNames = []string{"icao", "type", "title", "date", "cycle", "validity", "skip"}

var placeholderRe = regexp.MustCompile(`\{([^{}]*)\}`)

// ParsePattern compiles a pattern. The error says, in plain words, what is wrong with it.
func ParsePattern(text string) (*Pattern, error) {
	text = strings.TrimSpace(text)
	if strings.EqualFold(text[max(0, len(text)-4):], ".pdf") {
		text = strings.TrimSpace(text[:len(text)-4]) // names are matched without the extension
	}
	if text == "" {
		return nil, fmt.Errorf("the pattern is empty")
	}
	if len(text) > 200 {
		return nil, fmt.Errorf("the pattern is too long")
	}
	p := &Pattern{text: text, dates: map[int]string{}}
	var re strings.Builder
	re.WriteString(`(?i)^`)
	seen := map[string]bool{}
	useful := false
	group := 0
	pos := 0
	for _, loc := range placeholderRe.FindAllStringSubmatchIndex(text, -1) {
		re.WriteString(literalRegexp(text[pos:loc[0]]))
		pos = loc[1]
		name, arg, _ := strings.Cut(strings.TrimSpace(text[loc[2]:loc[3]]), ":")
		name = strings.ToLower(strings.TrimSpace(name))
		arg = strings.TrimSpace(arg)
		switch name {
		case "icao", "type", "title":
			if arg != "" {
				return nil, fmt.Errorf("{%s} takes no format", name)
			}
		case "date", "validity":
			name = "date"
		case "cycle", "airac":
			name = "cycle"
			if arg != "" {
				return nil, fmt.Errorf("{cycle} takes no format")
			}
		case "skip", "*", "ignore":
			name = "skip"
		default:
			return nil, fmt.Errorf("unknown placeholder {%s}: use %s", text[loc[2]:loc[3]], "{icao}, {type}, {title}, {date} or {skip}")
		}
		if name == "cycle" {
			name = "date" // a chart has one validity: a cycle and a date are the same field
			p.cycleGroups = append(p.cycleGroups, group+1)
		}
		if name != "skip" {
			if seen[name] {
				return nil, fmt.Errorf("{%s} appears twice", name)
			}
			seen[name] = true
			useful = true
		}
		switch name {
		case "icao":
			re.WriteString(`([a-z]{4})`)
		case "type":
			re.WriteString(`([a-z0-9]+)`)
		case "title":
			re.WriteString(`(.+?)`)
		case "skip":
			re.WriteString(`.*?`)
		case "date":
			expr, err := dateRegexp(arg)
			if len(p.cycleGroups) > 0 && p.cycleGroups[len(p.cycleGroups)-1] == group+1 {
				expr, err = `\d{4}`, nil
			}
			if err != nil {
				return nil, err
			}
			re.WriteString(`(` + expr + `)`)
		}
		if name != "skip" {
			group++
			p.fields = append(p.fields, name)
			if name == "date" {
				p.dates[group] = arg
			}
		}
	}
	re.WriteString(literalRegexp(text[pos:]))
	re.WriteString(`$`)
	if !useful {
		return nil, fmt.Errorf("the pattern needs at least one of {icao}, {type}, {title} or {date}")
	}
	compiled, err := regexp.Compile(re.String())
	if err != nil {
		return nil, fmt.Errorf("the pattern cannot be used: %v", err)
	}
	p.re = compiled
	return p, nil
}

// literalRegexp matches literal text: exactly, but a separator may be any of "_", "-" or a space.
func literalRegexp(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '_', '-', ' ':
			b.WriteString(`[_\- ]`)
		default:
			b.WriteString(regexp.QuoteMeta(string(r)))
		}
	}
	return b.String()
}

// dateRegexp turns a date format ("DD-MM-YYYY") into a regular expression; "" is automatic.
func dateRegexp(format string) (string, error) {
	if format == "" {
		return `\d{8}|\d{4}[-_.]\d{2}[-_.]\d{2}`, nil
	}
	if !strings.Contains(format, "YY") || !strings.Contains(format, "MM") || !strings.Contains(format, "DD") {
		return "", fmt.Errorf("the date format %q needs YYYY (or YY), MM and DD", format)
	}
	var b strings.Builder
	f := strings.ToUpper(format)
	for i := 0; i < len(f); {
		switch {
		case strings.HasPrefix(f[i:], "YYYY"):
			b.WriteString(`\d{4}`)
			i += 4
		case strings.HasPrefix(f[i:], "YY"):
			b.WriteString(`\d{2}`)
			i += 2
		case strings.HasPrefix(f[i:], "MM"):
			b.WriteString(`\d{2}`)
			i += 2
		case strings.HasPrefix(f[i:], "DD"):
			b.WriteString(`\d{2}`)
			i += 2
		default:
			b.WriteString(literalRegexp(f[i : i+1]))
			i++
		}
	}
	return b.String(), nil
}

func (p *Pattern) isCycleGroup(g int) bool {
	for _, c := range p.cycleGroups {
		if c == g {
			return true
		}
	}
	return false
}

// Text returns the pattern as the user wrote it.
func (p *Pattern) Text() string { return p.text }

// Match reads the attributes out of a file name (with or without ".pdf").
func (p *Pattern) Match(name string) (Info, bool) {
	stem := strings.TrimSpace(name)
	if strings.EqualFold(stem[max(0, len(stem)-4):], ".pdf") {
		stem = stem[:len(stem)-4]
	}
	m := p.re.FindStringSubmatch(stem)
	if m == nil {
		return Info{}, false
	}
	var in Info
	for i, field := range p.fields {
		v := strings.TrimSpace(m[i+1])
		switch field {
		case "icao":
			in.ICAO = strings.ToUpper(v)
		case "type":
			in.Type = strings.ToUpper(v)
		case "title":
			in.Title = Tidy(v)
		case "date":
			if p.isCycleGroup(i + 1) {
				in.Validity, _ = DateOfCycle(v)
			} else {
				in.Validity = parseDateWith(v, p.dates[i+1])
			}
		}
	}
	// "sbsp-adc" style titles repeat what the other fields already say
	if in.Title != "" && strings.EqualFold(in.Title, strings.TrimSpace(in.ICAO+" "+in.Type)) {
		in.Title = ""
	}
	return in, true
}

// parseDateWith reads a date written in the given format ("" = YYYYMMDD or YYYY-MM-DD).
func parseDateWith(value, format string) string {
	digits := func(s string) string {
		var b strings.Builder
		for _, r := range s {
			if r >= '0' && r <= '9' {
				b.WriteRune(r)
			}
		}
		return b.String()
	}
	if format == "" {
		return ParseDate(digits(value))
	}
	f := strings.ToUpper(format)
	var y, mo, d string
	vi := 0
	for i := 0; i < len(f) && vi <= len(value); {
		take := func(n int) string {
			if vi+n > len(value) {
				return ""
			}
			s := value[vi : vi+n]
			vi += n
			return s
		}
		switch {
		case strings.HasPrefix(f[i:], "YYYY"):
			y = take(4)
			i += 4
		case strings.HasPrefix(f[i:], "YY"):
			if v := take(2); v != "" {
				y = "20" + v
			}
			i += 2
		case strings.HasPrefix(f[i:], "MM"):
			mo = take(2)
			i += 2
		case strings.HasPrefix(f[i:], "DD"):
			d = take(2)
			i += 2
		default:
			vi++
			i++
		}
	}
	return ParseDate(y + mo + d)
}
