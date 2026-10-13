package server

import (
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/importer"
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
	mux.HandleFunc("POST /api/packages/upload", s.handlePackageUpload)
	mux.HandleFunc("GET /api/sources/faa-us/metadata", s.handleFAAStatus)
	mux.HandleFunc("POST /api/sources/faa-us/metadata", s.handleFAAMetadata)
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
	if errors.Is(err, sources.ErrFAAMetadataArchive) {
		// not charts: the metadata of the cycle. The UI hands it to the metadata endpoint.
		writeJSON(w, 200, map[string]any{"kind": "metadata", "path": strings.TrimSpace(r.URL.Query().Get("path"))})
		return
	}
	if err != nil {
		status := 400
		if errors.Is(err, sources.ErrNeedsFAAMetadata) {
			status = 409
		}
		if errors.Is(err, os.ErrNotExist) {
			status = 404
		}
		writeErr(w, status, err)
		return
	}
	// what is new to this library, and how long it would take here
	newBy := map[string]int{}
	total := 0
	for _, e := range p.Entries() {
		if _, ok := s.lib.FindByOriginal(e.File); !ok {
			newBy[e.ICAO]++
			total++
		}
	}
	per := s.impTime.secondsPerChart()
	writeJSON(w, 200, map[string]any{
		"path": p.Path, "size": p.Size, "modified": p.Modified, "date": p.Date, "charts": p.Charts, "airports": p.Airports,
		"new": total, "newByAirport": newBy, "secondsPerChart": per, "estimateSeconds": int(float64(total)*per + 0.5),
	})
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
		All   bool     `json:"all"` // every chart of the package that is not in the library yet
		// NoAirport selects the charts with no airport (VFR and IFR rasters, visual charts).
		NoAirport bool `json:"noAirport"`
	}
	if err := readJSON(r, &body); err != nil {
		writeErr(w, 400, err)
		return
	}
	if strings.TrimSpace(body.ICAO) == "" && !body.All && !body.NoAirport {
		writeErr(w, 400, errors.New("icao is required, or ask for all"))
		return
	}
	pkg, err := pk.OpenPackage(strings.TrimSpace(body.Path))
	if err != nil {
		writeErr(w, 400, err)
		return
	}
	var entries []sources.Entry
	if body.All {
		// only what is not in the library: the job total and its estimate are then true
		for _, e := range pkg.Entries() {
			if _, have := s.lib.FindByOriginal(e.File); !have {
				entries = append(entries, e)
			}
		}
		if len(entries) == 0 {
			writeErr(w, 409, errors.New("everything in this package is already in the library"))
			return
		}
	} else {
		entries = pkg.Select(body.ICAO, body.Types)
	}
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
			form := importForm{Source: srcID, Package: filepath.Base(pkg.Path)}
			if e.Sidecar != "" { // the .htm of a raster: its publication date becomes the validity
				if htm, err := pkg.ReadMember(e.Sidecar); err == nil {
					form.Validity = importer.ParseFGDCPublication(htm)
				}
			}
			c, err := s.importPDF(data, e.File, form)
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

// ------------------------------------------------------------------ uploaded packages

// Packages the user chose in the browser are kept here: importing an aerodrome reads the archive again.
func (s *Server) packagesDir() string { return filepath.Join(s.lib.Dir(), "packages") }

// maxPackageUpload bounds one uploaded package (a full one is some hundreds of MB).
const maxPackageUpload = 2 << 30

var unsafeNameChars = regexp.MustCompile(`[<>:"/\\|?*\x00-\x1f]`)

// packageFileName turns a client file name into one safe to create in the packages folder: its base
// name only, no characters Windows refuses, and a .zip extension. It returns "" for anything else.
func packageFileName(raw string) string {
	name := raw
	if i := strings.LastIndexAny(name, `/\`); i >= 0 {
		name = name[i+1:]
	}
	name = strings.TrimSpace(unsafeNameChars.ReplaceAllString(name, "_"))
	name = strings.Trim(name, ". ")
	if strings.EqualFold(filepath.Ext(name), ".zip") && len(name) > 4 && len(name) <= 200 {
		return name
	}
	return ""
}

// handlePackageUpload receives one ZIP package and keeps it in the packages folder, so what the user
// downloaded can be added from the browser. The body is streamed to disk (a package is big).
// The answer says where the file is, so the UI can read the package and import from it.
func (s *Server) handlePackageUpload(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxPackageUpload)
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
		name := packageFileName(part.FileName())
		if name == "" {
			writeErr(w, 400, errors.New("only .zip packages can be added here"))
			return
		}
		dir := s.packagesDir()
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
		writeJSON(w, 201, map[string]any{"name": name, "path": dst, "size": n})
		return
	}
	writeErr(w, 400, errors.New("no file in the request"))
}
