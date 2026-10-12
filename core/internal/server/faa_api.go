package server

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/sources"
)

// The FAA's chart files are named by number; the metadata of the cycle says what each one is
// (see sources/faa.go). It is kept next to the library and read again when the program starts.
func (s *Server) faaMetadataPath() string { return filepath.Join(s.lib.Dir(), "faa-metadata.json") }

func (s *Server) loadFAAMetadata() {
	b, err := os.ReadFile(s.faaMetadataPath())
	if err != nil {
		return
	}
	var m sources.FAAMetadata
	if json.Unmarshal(b, &m) == nil && len(m.Charts) > 0 {
		sources.SetFAAMetadata(&m)
	}
}

func (s *Server) saveFAAMetadata(m *sources.FAAMetadata) error {
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}
	tmp := s.faaMetadataPath() + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, s.faaMetadataPath()); err != nil {
		return err
	}
	sources.SetFAAMetadata(m)
	return nil
}

func faaStatus() map[string]any {
	m := sources.CurrentFAAMetadata()
	if m == nil {
		return map[string]any{"loaded": false}
	}
	return map[string]any{"loaded": true, "cycle": m.Cycle, "from": m.From, "to": m.To, "charts": len(m.Charts), "airports": m.Airports()}
}

func (s *Server) handleFAAStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, faaStatus())
}

// handleFAAMetadata takes the d-TPP metafile: the XML itself (multipart "file"), or the path of a
// zip already kept in the packages folder (the "E" set), given as JSON {"path": ...}.
func (s *Server) handleFAAMetadata(w http.ResponseWriter, r *http.Request) {
	var m *sources.FAAMetadata
	var err error
	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/") {
		r.Body = http.MaxBytesReader(w, r.Body, 64<<20)
		file, _, ferr := r.FormFile("file")
		if ferr != nil {
			writeErr(w, 400, errors.New("no file in the request"))
			return
		}
		defer file.Close()
		m, err = sources.ParseFAAMetafile(io.LimitReader(file, 64<<20))
	} else {
		var body struct {
			Path string `json:"path"`
		}
		if rerr := readJSON(r, &body); rerr != nil {
			writeErr(w, 400, rerr)
			return
		}
		// only archives the program itself keeps: never an arbitrary path of the computer
		p := filepath.Clean(body.Path)
		if filepath.Dir(p) != filepath.Clean(s.packagesDir()) {
			writeErr(w, 400, errors.New("the file must be one added through the app"))
			return
		}
		m, err = sources.ReadFAAMetafileFromZip(p)
	}
	if err != nil {
		writeErr(w, 400, err)
		return
	}
	if err := s.saveFAAMetadata(m); err != nil {
		writeErr(w, 500, err)
		return
	}
	writeJSON(w, 200, faaStatus())
}
