package importer

import (
	"fmt"

	"github.com/klippa-app/go-pdfium/requests"
)

// PageText returns the text of the first page of a PDF. Charts of some providers carry their own
// reference in it (a footer such as "AD 2-EGLL-6-1"), which is how they are recognised when the file
// name says nothing.
func (r *Renderer) PageText(pdf []byte) (string, error) {
	doc, err := r.instance.OpenDocument(&requests.OpenDocument{File: &pdf})
	if err != nil {
		return "", fmt.Errorf("open: %w", err)
	}
	defer r.instance.FPDF_CloseDocument(&requests.FPDF_CloseDocument{Document: doc.Document})
	res, err := r.instance.GetPageText(&requests.GetPageText{
		Page: requests.Page{ByIndex: &requests.PageByIndex{Document: doc.Document, Index: 0}},
	})
	if err != nil {
		return "", err
	}
	return res.Text, nil
}
