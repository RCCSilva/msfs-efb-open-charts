package server

import (
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/jobs"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/sources"
)

func (s *Server) registerSourceRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/sources", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, sources.All()) })
	mux.HandleFunc("GET /api/sources/{id}/catalog", s.handleCatalog)
	mux.HandleFunc("GET /api/sources/{id}/locate", s.handleLocate)
	mux.HandleFunc("GET /api/sources/{id}/package", s.handlePackageInfo)
	mux.HandleFunc("POST /api/sources/{id}/package/import", s.handlePackageImport)
	mux.HandleFunc("GET /api/jobs/{id}", s.handleJob)
	mux.HandleFunc("GET /api/inbox", s.handleInbox)
	mux.HandleFunc("POST /api/inbox/scan", s.handleInboxScan)
	mux.HandleFunc("POST /api/inbox/upload", s.handleInboxUpload)
}

// catalogRow is a catalog item plus whether it is already in the library.
type catalogRow struct {
	sources.CatalogItem
	Installed bool   `json:"installed"`
	ChartID   string `json:"chartId,omitempty"`
}

func (s *Server) handleCatalog(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	info, ok := sources.Get(id)
	if !ok {
		writeErr(w, 404, errors.New("unknown source"))
		return
	}
	q := r.URL.Query()
	items := sources.Catalog(id, sources.CatalogFilter{Text: q.Get("q"), Type: q.Get("type")})
	limit := 300
	if n, err := strconv.Atoi(q.Get("limit")); err == nil && n > 0 {
		limit = n
	}
	rows := make([]catalogRow, 0, min(len(items), limit))
	for _, it := range items {
		if len(rows) >= limit {
			break
		}
		row := catalogRow{CatalogItem: it}
		if c, ok := s.lib.FindByOriginal(it.File); ok {
			row.Installed, row.ChartID = true, c.ID
		}
		rows = append(rows, row)
	}
	writeJSON(w, 200, map[string]any{
		"source": info.ID, "snapshot": info.CatalogDate, "policy": info.Policy,
		"total": len(items), "items": rows,
	})
}

// handleLocate returns the dynamic addresses of a source for a date (default: today).
// Nothing is fetched: the user opens the address.
func (s *Server) handleLocate(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	info, ok := sources.Get(id)
	if !ok {
		writeErr(w, 404, errors.New("unknown source"))
		return
	}
	on := time.Now()
	if d := strings.TrimSpace(r.URL.Query().Get("date")); d != "" {
		t, err := time.Parse("2006-01-02", d)
		if err != nil {
			writeErr(w, 400, errors.New("date must be YYYY-MM-DD"))
			return
		}
		on = t
	}
	locs := sources.Locate(id, on)
	if locs == nil {
		locs = []sources.Location{}
	}
	writeJSON(w, 200, map[string]any{"source": info.ID, "date": on.Format("2006-01-02"), "homepage": info.Homepage, "locations": locs})
}

func packagerFor(w http.ResponseWriter, id string) (sources.Packager, bool) {
	if _, ok := sources.Get(id); !ok {
		writeErr(w, 404, errors.New("unknown source"))
		return nil, false
	}
	p, ok := sources.PackagerFor(id)
	if !ok {
		writeErr(w, 400, errors.New("this source has no package format"))
		return nil, false
	}
	return p, true
}

func (s *Server) handlePackageInfo(w http.ResponseWriter, r *http.Request) {
	pk, ok := packagerFor(w, r.PathValue("id"))
	if !ok {
		return
	}
	p, err := pk.OpenPackage(strings.TrimSpace(r.URL.Query().Get("path")))
	if err != nil {
		status := 400
		if errors.Is(err, os.ErrNotExist) {
			status = 404
		}
		writeErr(w, status, err)
		return
	}
	writeJSON(w, 200, p)
}

func (s *Server) handlePackageImport(w http.ResponseWriter, r *http.Request) {
	if !s.canImport() {
		writeErr(w, 503, errors.New("importer unavailable"))
		return
	}
	srcID := r.PathValue("id")
	pk, ok := packagerFor(w, srcID)
	if !ok {
		return
	}
	var body struct {
		Path  string   `json:"path"`
		ICAO  string   `json:"icao"`
		Types []string `json:"types"`
	}
	if err := readJSON(r, &body); err != nil {
		writeErr(w, 400, err)
		return
	}
	if strings.TrimSpace(body.ICAO) == "" {
		writeErr(w, 400, errors.New("icao is required: import one airport at a time"))
		return
	}
	pkg, err := pk.OpenPackage(strings.TrimSpace(body.Path))
	if err != nil {
		writeErr(w, 400, err)
		return
	}
	entries := pkg.Select(body.ICAO, body.Types)
	if len(entries) == 0 {
		writeErr(w, 404, errors.New("no charts for that airport/type in the package"))
		return
	}
	job := s.jobs.Start("package-import", len(entries), func(j *jobs.Job) {
		for _, e := range entries {
			j.SetCurrent(e.File)
			data, err := pkg.Read(e)
			if err != nil {
				j.Add(jobs.Item{File: e.File, Status: "error", Message: err.Error()})
				continue
			}
			c, err := s.importPDF(data, e.File, importForm{Source: srcID, Package: filepath.Base(pkg.Path)})
			if err != nil {
				st, msg := itemStatus(err)
				it := jobs.Item{File: e.File, Status: st, Message: msg}
				if c != nil {
					it.ChartID, it.Name = c.ID, c.Name
				}
				j.Add(it)
				continue
			}
			j.Add(jobs.Item{File: e.File, ChartID: c.ID, Name: c.Name, Status: "ok"})
		}
	})
	snap, _ := s.jobs.Get(job.ID())
	writeJSON(w, 202, snap)
}

func (s *Server) handleJob(w http.ResponseWriter, r *http.Request) {
	j, ok := s.jobs.Get(r.PathValue("id"))
	if !ok {
		writeErr(w, 404, errors.New("job not found"))
		return
	}
	writeJSON(w, 200, j)
}

// ------------------------------------------------------------------ inbox

// The inbox is a folder the user drops files into. It is only read when the user asks.
func (s *Server) inboxDir() string { return filepath.Join(s.lib.Dir(), "inbox") }

type inboxFile struct {
	Name     string    `json:"name"`
	Path     string    `json:"path"`
	Size     int64     `json:"size"`
	Modified time.Time `json:"modified"`
	Kind     string    `json:"kind"` // "pdf", "zip"
}

func (s *Server) listInbox() []inboxFile {
	dir := s.inboxDir()
	_ = os.MkdirAll(dir, 0o755)
	entries, _ := os.ReadDir(dir)
	var out []inboxFile
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		kind := ""
		switch strings.ToLower(filepath.Ext(e.Name())) {
		case ".pdf":
			kind = "pdf"
		case ".zip":
			kind = "zip"
		default:
			continue
		}
		fi, err := e.Info()
		if err != nil {
			continue
		}
		out = append(out, inboxFile{Name: e.Name(), Path: filepath.Join(dir, e.Name()), Size: fi.Size(), Modified: fi.ModTime(), Kind: kind})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func (s *Server) handleInbox(w http.ResponseWriter, r *http.Request) {
	files := s.listInbox()
	if files == nil {
		files = []inboxFile{}
	}
	writeJSON(w, 200, map[string]any{"dir": s.inboxDir(), "files": files})
}

// handleInboxScan imports every PDF in the inbox (moving it to inbox/done). Zips are only
// reported: the user chooses what to import from them.
func (s *Server) handleInboxScan(w http.ResponseWriter, r *http.Request) {
	if !s.canImport() {
		writeErr(w, 503, errors.New("importer unavailable"))
		return
	}
	source := strings.TrimSpace(r.URL.Query().Get("source"))
	if source == "" {
		source = "manual"
	}
	files := s.listInbox()
	var pdfs []inboxFile
	zips := []inboxFile{}
	for _, f := range files {
		if f.Kind == "pdf" {
			pdfs = append(pdfs, f)
		} else {
			zips = append(zips, f)
		}
	}
	if len(pdfs) == 0 {
		writeJSON(w, 200, map[string]any{"job": nil, "zips": zips, "message": "no PDFs in the inbox"})
		return
	}
	done := filepath.Join(s.inboxDir(), "done")
	_ = os.MkdirAll(done, 0o755)
	job := s.jobs.Start("inbox-scan", len(pdfs), func(j *jobs.Job) {
		for _, f := range pdfs {
			j.SetCurrent(f.Name)
			data, err := os.ReadFile(f.Path)
			if err != nil {
				j.Add(jobs.Item{File: f.Name, Status: "error", Message: err.Error()})
				continue
			}
			c, err := s.importPDF(data, f.Name, importForm{Source: source})
			if err != nil {
				st, msg := itemStatus(err)
				j.Add(jobs.Item{File: f.Name, Status: st, Message: msg})
				continue
			}
			_ = os.Rename(f.Path, filepath.Join(done, f.Name))
			j.Add(jobs.Item{File: f.Name, ChartID: c.ID, Name: c.Name, Status: "ok"})
		}
	})
	snap, _ := s.jobs.Get(job.ID())
	writeJSON(w, 202, map[string]any{"job": snap, "zips": zips})
}

// maxInboxUpload bounds one uploaded file: a chart is a few MB, a full package some hundreds.
const maxInboxUpload = 2 << 30

var unsafeNameChars = regexp.MustCompile(`[<>:"/\\|?*\x00-\x1f]`)

// inboxName turns a client file name into one safe to create in the inbox: its base name only, no
// characters Windows refuses, and a .pdf or .zip extension. It returns "" for anything else.
func inboxName(raw string) string {
	name := raw
	if i := strings.LastIndexAny(name, `/\`); i >= 0 {
		name = name[i+1:]
	}
	name = strings.TrimSpace(unsafeNameChars.ReplaceAllString(name, "_"))
	name = strings.Trim(name, ". ")
	switch strings.ToLower(filepath.Ext(name)) {
	case ".pdf", ".zip":
		if len(name) > 4 && len(name) <= 200 {
			return name
		}
	}
	return ""
}

// handleInboxUpload receives one PDF or ZIP and keeps it in the inbox folder, so what the user
// downloaded can be added from the browser. The body is streamed to disk (a package is big).
// The answer says where the file is, so the UI can read the package or import the PDF.
func (s *Server) handleInboxUpload(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxInboxUpload)
	mr, err := r.MultipartReader()
	if err != nil {
		writeErr(w, 400, errors.New("expected a multipart upload"))
		return
	}
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			writeErr(w, 400, err)
			return
		}
		if part.FormName() != "file" {
			continue
		}
		name := inboxName(part.FileName())
		if name == "" {
			writeErr(w, 400, errors.New("only .pdf and .zip files can be added"))
			return
		}
		dir := s.inboxDir()
		if err := os.MkdirAll(dir, 0o755); err != nil {
			writeErr(w, 500, err)
			return
		}
		dst := filepath.Join(dir, name)
		tmp := dst + ".part"
		f, err := os.Create(tmp)
		if err != nil {
			writeErr(w, 500, err)
			return
		}
		n, err := io.Copy(f, part)
		cerr := f.Close()
		if err == nil {
			err = cerr
		}
		if err == nil {
			err = os.Rename(tmp, dst)
		}
		if err != nil {
			_ = os.Remove(tmp)
			writeErr(w, 500, err)
			return
		}
		kind := "pdf"
		if strings.EqualFold(filepath.Ext(name), ".zip") {
			kind = "zip"
		}
		writeJSON(w, 201, map[string]any{"name": name, "path": dst, "kind": kind, "size": n})
		return
	}
	writeErr(w, 400, errors.New("no file in the request"))
}
