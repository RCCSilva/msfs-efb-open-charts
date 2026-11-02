package sources

import "github.com/RCCSilva/msfs-efb-open-charts/core/internal/naming"

// ChartFox: a chart service with an official API (see package chartfox). It is the one source whose charts the app brings
// itself, only when the user asks (an airport, then the charts picked), with the user's own account. Its files are
// named by the app (`<id>.pdf`) and carry the airport, type and title the API gave, so nothing is read from the name.
type chartfox struct{}

func (chartfox) Info() Info {
	return Info{
		ID:       "chartfox",
		Homepage: "https://chartfox.org",
		HomepageLabel: Text{
			PT: "ChartFox",
			EN: "ChartFox",
		},
		HomepageNote: Text{
			PT: "conecte a sua conta e procure as cartas por aeroporto nesta página",
			EN: "connect your account and look charts up by airport on this page",
		},
		Name: Text{PT: "ChartFox", EN: "ChartFox"},
		Description: Text{
			PT: "Cartas de muitos países, por aeroporto, pela API oficial do ChartFox.",
			EN: "Charts of many countries, by airport, through the official ChartFox API.",
		},
		Policy: Policy{
			AutomatedFetch: false, // never on its own: every download starts from the user
			UserDownloads:  false, // the app brings the files through the API
			Redistribution: false,
			URL:            "https://chartfox.org/legal/api-terms",
			Notes: Text{
				PT: "Cache só neste computador, sem redistribuir.",
				EN: "Cached only on this computer, never redistributed.",
			},
		},
	}
}

// Recognize: the app names these files itself and passes the metadata, there is nothing to read.
func (chartfox) Recognize(string) (naming.Info, bool) { return naming.Info{}, false }
