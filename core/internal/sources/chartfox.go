package sources

import "github.com/RCCSilva/msfs-efb-open-charts/core/internal/naming"

// ChartFox: a chart service with an official API (see package chartfox). It is the one source whose charts the app brings
// itself, only when the user asks (an airport, then the charts picked), with the user's own account. Its files are
// named by the app (`chartfox-<id>.pdf`) and carry the airport, type and title the API gave, so nothing is read from the name.
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
			PT: "conecte a sua conta no Setup e procure as cartas por aeroporto",
			EN: "connect your account in Setup and look charts up by airport",
		},
		Name: Text{PT: "ChartFox", EN: "ChartFox"},
		Description: Text{
			PT: "Cartas de muitos países, trazidas pela API oficial do ChartFox com a sua conta, só quando você pedir. Chart data powered by ChartFox.",
			EN: "Charts of many countries, brought through the official ChartFox API with your account, only when you ask. Chart data powered by ChartFox.",
		},
		Policy: Policy{
			AutomatedFetch: false, // never on its own: every download starts from the user
			UserDownloads:  false, // the app brings the files through the API
			Redistribution: false,
			URL:            "https://chartfox.org/legal/api-terms",
			Notes: Text{
				PT: "Uso pela API oficial, autorizado pelo ChartFox: sob demanda, com a sua conta, em cache só neste computador, sem redistribuir. Apenas para simulação de voo.",
				EN: "Used through the official API, authorized by ChartFox: on demand, with your account, cached only on this computer, never redistributed. For flight simulation only.",
			},
		},
	}
}

// Recognize: the app names these files itself and passes the metadata, there is nothing to read.
func (chartfox) Recognize(string) (naming.Info, bool) { return naming.Info{}, false }
