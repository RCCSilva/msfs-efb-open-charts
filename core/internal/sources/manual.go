package sources

import "github.com/RCCSilva/msfs-efb-open-charts/core/internal/naming"

// manual is the built-in source: any georeferenced PDF the user brings (upload or inbox
// folder). It has no provider, so it has no catalog, no locator and no archive format.
type manual struct{}

func (manual) Info() Info {
	return Info{
		ID:      "manual",
		Builtin: true,
		Name:    Text{PT: "Meus PDFs", EN: "My PDFs"},
		Description: Text{
			PT: "Envie qualquer carta em PDF georreferenciado. Você pode ajustar ICAO, tipo e nome.",
			EN: "Add any georeferenced chart PDF. You can set the ICAO, type and name.",
		},
		Policy: Policy{
			UserDownloads: true,
			Notes: Text{
				PT: "Os arquivos ficam só no seu computador.",
				EN: "Files stay on your computer only.",
			},
		},
		Guide: []Text{
			{PT: "Arraste o PDF para a janela, ou coloque-o na pasta inbox.", EN: "Drop the PDF on the window, or put it in the inbox folder."},
			{PT: "O PDF precisa ser georreferenciado (GeoPDF).", EN: "The PDF must be georeferenced (GeoPDF)."},
		},
	}
}

// Recognize never claims a name: the manual source has no naming rules of its own. Names that
// look like a known provider's are recognised through Recognize at registry level.
func (manual) Recognize(string) (naming.Info, bool) { return naming.Info{}, false }
