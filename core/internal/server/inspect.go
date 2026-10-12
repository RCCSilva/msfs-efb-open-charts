package server

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/naming"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/pdfgeo"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/sources"
)

// A PDF the user dropped is first inspected: the app proposes ICAO, type, title and name and the
// user confirms or edits them before anything is imported. The bytes wait here under a token.
type staged struct {
	data     []byte
	filename string
	at       time.Time
}

type stagedStore struct {
	mu sync.Mutex
	m  map[string]*staged
}

const (
	stageTTL = 30 * time.Minute
	stageMax = 40
)

func (st *stagedStore) put(data []byte, filename string) string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	tok := hex.EncodeToString(b)
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.m == nil {
		st.m = map[string]*staged{}
	}
	for k, v := range st.m {
		if time.Since(v.at) > stageTTL {
			delete(st.m, k)
		}
	}
	for len(st.m) >= stageMax { // drop the oldest
		var oldest string
		for k, v := range st.m {
			if oldest == "" || v.at.Before(st.m[oldest].at) {
				oldest = k
			}
		}
		delete(st.m, oldest)
	}
	st.m[tok] = &staged{data: data, filename: filename, at: time.Now()}
	return tok
}

func (st *stagedStore) get(tok string) (*staged, bool) {
	st.mu.Lock()
	defer st.mu.Unlock()
	v, ok := st.m[tok]
	return v, ok
}

func (st *stagedStore) drop(tok string) {
	st.mu.Lock()
	delete(st.m, tok)
	st.mu.Unlock()
}

type suggestion struct {
	ICAO     string `json:"icao"`
	Type     string `json:"type"`
	Title    string `json:"title"`
	Validity string `json:"validity"`
	Name     string `json:"name"`
}

// handleInspect reads a PDF without importing it and proposes its metadata.
func (s *Server) handleInspect(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<20)
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		writeErr(w, 400, err)
		return
	}
	file, hdr, err := r.FormFile("file")
	if err != nil {
		writeErr(w, 400, errors.New("missing form field \"file\""))
		return
	}
	defer file.Close()
	data, err := io.ReadAll(file)
	if err != nil {
		writeErr(w, 400, err)
		return
	}
	if len(data) < 8 || string(data[:5]) != "%PDF-" {
		writeErr(w, 400, errors.New("not a PDF file"))
		return
	}
	source := strings.TrimSpace(r.FormValue("source"))
	if source == "" {
		source = "manual"
	}
	base := filepath.Base(hdr.Filename)
	stem := strings.TrimSuffix(base, filepath.Ext(base))

	out := map[string]any{"file": base, "size": len(data), "source": source}
	info, by := sources.Recognize(source, stem)
	out["recognizedBy"] = by

	viewport, geo := "", false
	if pg, err := pdfgeo.Read(data); err == nil && len(pg.Viewports) > 0 {
		geo = true
		viewport = strings.TrimSpace(pg.Viewports[0].Name)
	}
	out["georeferenced"], out["viewport"] = geo, viewport

	matched := ""
	if it, ok := sources.MatchCatalog(firstNonEmpty(by, source), base); ok {
		info.Type, info.Title, info.Validity = it.Type, it.Name, it.Effective
		matched = "catalog"
	}
	sg := suggestion{ICAO: info.ICAO, Type: info.Type, Title: info.Title, Validity: info.Validity}
	if sg.ICAO == "" && viewport != "" {
		sg.Title = viewport // charts without an ICAO are better named by the PDF's own title
	}
	if sg.Title == "" {
		sg.Title = naming.Tidy(stem)
	}
	sg.Name = naming.Display(sg.ICAO, sg.Type, sg.Title)
	out["suggestion"] = sg
	out["matched"] = matched
	// confident: the file is a known provider file (catalog, or a package-style name with ICAO
	// and type), so the UI may import it without asking.
	out["confident"] = geo && (matched != "" || (by != "" && sg.ICAO != "" && sg.Type != ""))

	if c, ok := s.lib.FindByOriginal(base); ok {
		out["duplicate"] = map[string]string{"id": c.ID, "name": c.Name}
	}
	out["token"] = s.staged.put(data, base)
	writeJSON(w, 200, out)
}

func firstNonEmpty(v ...string) string {
	for _, s := range v {
		if s != "" {
			return s
		}
	}
	return ""
}
