package sources

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/naming"
)

// United Kingdom: the UK AIP of NATS (published through the "Aurora" system). The charts are PDFs
// of the eAIP whose file names are numbers ("graphics/475382.pdf"), so they cannot be told apart by
// name. Each chart carries its own reference in the page, a footer such as "AD 2-EGLL-6-1" (the
// aerodrome and the number of the chart in the aerodrome's section AD 2.24): that is what is read.
//
// The procedure charts (SID, STAR, approach) are georeferenced PDFs; aerodrome and ground-movement
// charts are not (they are imported as view-only).

const (
	ukHome     = "https://nats-uk.ead-it.com/cms-nats/opencms/en/Publications/AIP/"
	ukTerms    = "https://nats-uk.ead-it.com/cms-nats/opencms/en/agb/"
	ukEAIP     = "https://www.aurora.nats.co.uk/htmlAIP/Publications/%s-AIRAC/html/index-en-GB.html"
	ukVerified = "2026-10-15"
)

type uk struct{}

func (uk) Info() Info {
	return Info{
		ID:          "nats-uk",
		Country:     "GB",
		CountryName: Text{PT: "Reino Unido", EN: "United Kingdom"},
		Homepage:    ukHome,
		HomepageLabel: Text{
			PT: "NATS: AIP do Reino Unido",
			EN: "NATS: UK AIP",
		},
		HomepageNote: Text{
			PT: "abra a edição em vigor, a parte AD 2 e o aeródromo; abra a carta e salve o PDF",
			EN: "open the edition in force, part AD 2 and the aerodrome; open the chart and save the PDF",
		},
		Name: Text{PT: "NATS / AIP do Reino Unido", EN: "NATS / UK AIP"},
		Description: Text{
			PT: "Cartas do AIP do Reino Unido (NATS). Procedimentos (SID, STAR, aproximação) vêm georreferenciados; cartas de aeródromo entram só para visualização.",
			EN: "Charts of the UK AIP (NATS). Procedures (SID, STAR, approach) are georeferenced; aerodrome charts are view-only.",
		},
		Policy: Policy{
			AutomatedFetch: false,
			UserDownloads:  true,
			Redistribution: false,
			URL:            ukTerms,
			Notes: Text{
				PT: "Acesso gratuito, sujeito aos termos da NATS (precisão e responsabilidade). As páginas não trazem uma cláusula de cópia ou redistribuição, então o aplicativo não baixa nada sozinho e não distribui nada. Os endereços mudam a cada ciclo AIRAC.",
				EN: "Free access, under NATS terms (accuracy and liability). The pages carry no clause on copying or redistribution, so the app downloads nothing by itself and distributes nothing. The addresses change every AIRAC cycle.",
			},
		},
		Guide: []Text{
			{PT: "Abra o AIP do Reino Unido pelo link abaixo (ou pelo endereço do ciclo, mais abaixo) e entre na parte AD 2 do aeródromo que você quer.", EN: "Open the UK AIP through the link below (or the address of the cycle, further down) and go to part AD 2 of the aerodrome you want."},
			{PT: "Na seção AD 2.24 estão as cartas. Abra a carta e salve o PDF (os arquivos têm nomes numéricos, como 475382.pdf: não renomeie).", EN: "Section AD 2.24 lists the charts. Open the chart and save the PDF (the files have numeric names like 475382.pdf: do not rename them)."},
			{PT: "Solte os PDFs na área \"Adicione o que você baixou\". O aplicativo lê no rodapé da carta o código (por exemplo AD 2-EGLL-6-1) e descobre o aeródromo e o tipo sozinho.", EN: "Drop the PDFs in the \"Add what you downloaded\" area. The app reads the chart's own reference in the footer (for example AD 2-EGLL-6-1) and works out the aerodrome and the type by itself."},
			{PT: "Cartas de SID, STAR e aproximação são posicionadas no mapa. As demais (aeródromo, solo) entram só para visualização.", EN: "SID, STAR and approach charts are placed on the map. The others (aerodrome, ground) are view-only."},
		},
	}
}

// Recognize: the file names of this provider are numbers and say nothing.
func (uk) Recognize(string) (naming.Info, bool) { return naming.Info{}, false }

// Locate gives the address of the eAIP of the cycle in force and of the next one.
func (uk) Locate(on time.Time) []Location {
	cur, id := naming.AIRAC(on)
	next := cur.AddDate(0, 0, 28)
	_, nid := naming.AIRAC(next)
	notes := Text{
		PT: "Endereço confirmado no portal da NATS em " + ukVerified + ". Os links mudam a cada ciclo: não guarde nos favoritos.",
		EN: "Address confirmed on the NATS portal on " + ukVerified + ". The links change every cycle: do not bookmark them.",
	}
	return []Location{
		{Kind: "eaip", Label: Text{PT: "AIP do ciclo " + id + " (em vigor)", EN: "AIP of cycle " + id + " (in force)"}, Cycle: id,
			Effective: cur.Format("2006-01-02"), Current: true, URL: fmt.Sprintf(ukEAIP, cur.Format("2006-01-02")), Verified: true, Notes: notes},
		{Kind: "eaip", Label: Text{PT: "AIP do próximo ciclo " + nid, EN: "AIP of next cycle " + nid}, Cycle: nid,
			Effective: next.Format("2006-01-02"), URL: fmt.Sprintf(ukEAIP, next.Format("2006-01-02")), Verified: true, Notes: notes},
	}
}

var (
	ukCode    = regexp.MustCompile(`AD\s*2\s*-\s*([A-Z]{4})\s*-\s*(\d+)\s*-\s*(\d+)`)
	ukHeading = regexp.MustCompile(`(?i)^(STANDARD (DEPARTURE|ARRIVAL) CHART.*|INITIAL APPROACH PROCEDURES.*|INSTRUMENT APPROACH CHART.*|.*GROUND MOVEMENT.*|.*PARKING.*|AERODROME OBSTACLE CHART.*|AERODROME CHART.*|.*SURVEILLANCE MINIMUM ALTITUDE CHART.*)$`)
	ukRunway  = regexp.MustCompile(`RWY\s?\d{2}[LRC]?(?:/[LRC])*(?:\s+RWY\s?\d{2}[LRC]?(?:/[LRC])*)*`)
)

// ukOwnCode finds the chart's own reference among the references the text may cite ("see AD
// 2-EGLL-2-4"): the one printed after the date of the chart, or alone on its line, else the last.
// It returns the whole match, the aerodrome, the section and the number.
func ukOwnCode(text string) []string {
	all := ukCode.FindAllStringSubmatchIndex(text, -1)
	if all == nil {
		return nil
	}
	best, bestScore := -1, -1
	for i, loc := range all {
		score := 1
		before := text[max(0, loc[0]-40):loc[0]]
		if strings.Contains(strings.ToUpper(before), "AERO INFO DATE") {
			score = 3
		} else {
			start := strings.LastIndexAny(text[:loc[0]], "\r\n") + 1
			end := strings.IndexAny(text[loc[1]:], "\r\n")
			if end < 0 {
				end = len(text) - loc[1]
			}
			if strings.TrimSpace(text[start:loc[1]+end]) == strings.TrimSpace(text[loc[0]:loc[1]]) {
				score = 2
			}
		}
		if score >= bestScore {
			best, bestScore = i, score
		}
	}
	loc := all[best]
	return []string{text[loc[0]:loc[1]], text[loc[2]:loc[3]], text[loc[4]:loc[5]], text[loc[6]:loc[7]]}
}

// RecognizeContent reads the text of the first page of a chart of the UK AIP.
func (uk) RecognizeContent(text string) (naming.Info, bool) {
	m := ukOwnCode(text)
	if m == nil {
		return naming.Info{}, false
	}
	icao, sec, num := m[1], m[2], m[3]
	lines := strings.Split(strings.ReplaceAll(text, "\r", ""), "\n")
	var heading string
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if len(l) > 6 && len(l) < 120 && ukHeading.MatchString(l) {
			heading = l
			break
		}
	}
	// the title block that follows the footer code and the aerodrome's code:
	// "EGLL" + "AIRCRAFT GROUND MOVEMENT/ALL TAXIWAYS" + "CHART - ICAO", up to the line of the ARP
	if heading == "" {
		for i, l := range lines {
			if !ukCode.MatchString(l) {
				continue
			}
			j := i + 1
			for j < len(lines) && j < i+4 && strings.TrimSpace(lines[j]) != icao {
				j++
			}
			if j >= len(lines) || strings.TrimSpace(lines[j]) != icao {
				break
			}
			var block []string
			for _, n := range lines[j+1 : min(len(lines), j+6)] {
				n = strings.TrimSpace(n)
				u := strings.ToUpper(n)
				if strings.HasPrefix(u, "ARP") || strings.HasPrefix(u, "CHANGE") || strings.Contains(u, "ELEV") {
					break
				}
				if n != "" {
					block = append(block, n)
				}
			}
			if joined := strings.Join(block, " "); strings.Contains(strings.ToUpper(joined), "CHART") {
				heading = joined
			}
			break
		}
	}
	typ := ukType(heading)
	title := strings.TrimSpace(heading)
	if title != "" {
		if rw := ukRunway.FindString(strings.ToUpper(text)); rw != "" && (typ == "SID" || typ == "STAR" || typ == "IAC") && !strings.Contains(strings.ToUpper(title), "RWY") {
			title += " " + rw
		}
		title = naming.Tidy(title) + " (" + sec + "-" + num + ")"
	} else {
		title = "AD 2-" + icao + "-" + sec + "-" + num
	}
	return naming.Info{ICAO: icao, Type: typ, Title: title, Sure: typ != ""}, true
}

func ukType(heading string) string {
	h := strings.ToUpper(heading)
	switch {
	case strings.Contains(h, "STANDARD DEPARTURE"), strings.Contains(h, "(SID)"):
		return "SID"
	case strings.Contains(h, "STANDARD ARRIVAL"), strings.Contains(h, "(STAR)"):
		return "STAR"
	case strings.Contains(h, "APPROACH"):
		return "IAC"
	case strings.Contains(h, "GROUND MOVEMENT"), strings.Contains(h, "PARKING"), strings.Contains(h, "DOCKING"):
		return "GMC"
	case strings.Contains(h, "OBSTACLE"):
		return "AOC"
	case strings.Contains(h, "AERODROME CHART"):
		return "ADC"
	case strings.Contains(h, "SURVEILLANCE MINIMUM ALTITUDE"):
		return "MVA"
	}
	return ""
}
