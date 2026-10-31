package server

import (
	"context"
	"errors"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/chartfox"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/naming"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/store"
)

const (
	// chartfoxSource is the source id of the charts brought from ChartFox.
	chartfoxSource = "chartfox"
	// listingTTL is how long an airport's chart list is reused before ChartFox is asked again (the terms ask for caching).
	listingTTL = 10 * time.Minute
	// maxChartFoxBatch caps the charts brought by one request.
	maxChartFoxBatch = 20
)

var (
	chartIDRe = regexp.MustCompile(`^[0-9a-fA-F-]{8,40}$`)
)

// chartfoxState is what the server keeps about ChartFox: the account, the catalogue and the lists already fetched.
type chartfoxState struct {
	account chartfox.Connector
	catalog chartfox.Catalog

	mu       sync.Mutex
	listings map[string]chartfoxListing
}

type chartfoxListing struct {
	at      time.Time
	airport chartfox.Airport
	charts  []chartfox.Chart
}

// UseChartFox gives the server the user's ChartFox account and the catalogue it reads with. Without them the ChartFox
// endpoints answer that ChartFox is not available.
func (s *Server) UseChartFox(account chartfox.Connector, catalog chartfox.Catalog) {
	s.chartfox = &chartfoxState{account: account, catalog: catalog, listings: map[string]chartfoxListing{}}
}

func (s *Server) chartfoxOr404(w http.ResponseWriter) *chartfoxState {
	if s.chartfox == nil || s.chartfox.account == nil {
		http.Error(w, "ChartFox is not available", http.StatusNotFound)
		return nil
	}
	return s.chartfox
}

func (s *Server) handleChartFoxStatus(w http.ResponseWriter, r *http.Request) {
	if s.chartfox == nil || s.chartfox.account == nil {
		writeJSON(w, 200, chartfox.Status{})
		return
	}
	writeJSON(w, 200, s.chartfox.account.Status())
}

// handleChartFoxConnect starts a login: the authorization page opens in the user's browser and the answer comes back to
// the loopback listener of the login, not to this server. The status endpoint tells how it went.
func (s *Server) handleChartFoxConnect(w http.ResponseWriter, r *http.Request) {
	cf := s.chartfoxOr404(w)
	if cf == nil {
		return
	}
	if err := cf.account.Connect(); err != nil {
		writeErr(w, http.StatusConflict, err)
		return
	}
	writeJSON(w, http.StatusAccepted, cf.account.Status())
}

// handleChartFoxDisconnect forgets the account and removes the charts brought from ChartFox (the policy in AGENTS.md).
func (s *Server) handleChartFoxDisconnect(w http.ResponseWriter, r *http.Request) {
	cf := s.chartfoxOr404(w)
	if cf == nil {
		return
	}
	if err := cf.account.Disconnect(); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	cf.mu.Lock()
	cf.listings = map[string]chartfoxListing{}
	cf.mu.Unlock()
	var ids []string
	for _, c := range s.lib.Search(store.Query{Source: chartfoxSource}) {
		ids = append(ids, c.ID)
	}
	removed, err := s.lib.DeleteMany(ids)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	st := cf.account.Status()
	writeJSON(w, 200, map[string]any{"available": st.Available, "connected": st.Connected, "connecting": st.Connecting, "removed": removed})
}

// chartfoxChart is a chart of an airport as the manager shows it.
type chartfoxChart struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Code     string `json:"code,omitempty"`
	Type     string `json:"type"`    // the app's code ("IAC", "SID"...), "" when ChartFox does not say
	TypeKey  string `json:"typeKey"` // ChartFox's own name for the type
	Georef   bool   `json:"georeferenced"`
	Imported bool   `json:"imported"` // already in the library
}

// chartfoxFile is the name the original file of a ChartFox chart has in the library.
func chartfoxFile(id string) string { return "chartfox-" + strings.ToLower(id) + ".pdf" }

// chartfoxStatus maps an error of the ChartFox client to the answer of the manager.
func chartfoxStatus(err error) int {
	switch {
	case errors.Is(err, chartfox.ErrUnauthorized):
		return http.StatusUnauthorized
	case errors.Is(err, chartfox.ErrNotFound):
		return http.StatusNotFound
	}
	return http.StatusBadGateway
}

// handleChartFoxAirport lists the charts of an airport. The list is kept for a few minutes so looking again, or
// importing, does not ask ChartFox again.
func (s *Server) handleChartFoxAirport(w http.ResponseWriter, r *http.Request) {
	cf := s.chartfoxOr404(w)
	if cf == nil {
		return
	}
	icao := strings.ToUpper(r.PathValue("icao"))
	if !icaoRe.MatchString(icao) {
		writeErr(w, http.StatusBadRequest, errors.New("not an airport code"))
		return
	}
	listing, err := s.chartfoxListing(r.Context(), cf, icao)
	if err != nil {
		writeErr(w, chartfoxStatus(err), err)
		return
	}
	charts := make([]chartfoxChart, 0, len(listing.charts))
	for _, c := range listing.charts {
		_, have := s.lib.FindByOriginal(chartfoxFile(c.ID))
		charts = append(charts, chartfoxChart{ID: c.ID, Name: c.Name, Code: c.Code, Type: chartfox.Kind(c.TypeKey), TypeKey: c.TypeKey, Georef: c.Georef, Imported: have})
	}
	writeJSON(w, 200, map[string]any{"airport": listing.airport, "charts": charts, "credit": chartfox.Credit, "creditURL": chartfox.CreditURL})
}

func (s *Server) chartfoxListing(ctx context.Context, cf *chartfoxState, icao string) (chartfoxListing, error) {
	cf.mu.Lock()
	l, ok := cf.listings[icao]
	cf.mu.Unlock()
	if ok && time.Since(l.at) < listingTTL {
		return l, nil
	}
	airport, err := cf.catalog.Airport(ctx, icao)
	if err != nil {
		return chartfoxListing{}, err
	}
	charts, err := cf.catalog.Charts(ctx, icao)
	if err != nil {
		return chartfoxListing{}, err
	}
	l = chartfoxListing{at: time.Now(), airport: airport, charts: charts}
	cf.mu.Lock()
	cf.listings[icao] = l
	cf.mu.Unlock()
	return l, nil
}

// chartfoxResult is the outcome for one chart of an import request.
type chartfoxResult struct {
	ID      string `json:"id"`
	Status  string `json:"status"` // "ok", "already" or "error"
	Message string `json:"message,omitempty"`
	ChartID string `json:"chartId,omitempty"` // the chart in the library
}

// errNotConnected is what an import reports when ChartFox refused the token: the rest of the batch is not tried.
const errNotConnected = "not connected to ChartFox"

// handleChartFoxImport brings the charts the user picked: for each, its detail, its PDF and the same import as any file.
// Nothing here runs unless the user asked; the batch is small.
func (s *Server) handleChartFoxImport(w http.ResponseWriter, r *http.Request) {
	cf := s.chartfoxOr404(w)
	if cf == nil {
		return
	}
	var body struct {
		IDs []string `json:"ids"`
	}
	if err := readJSON(r, &body); err != nil || len(body.IDs) == 0 {
		writeErr(w, http.StatusBadRequest, errors.New("pick at least one chart"))
		return
	}
	if len(body.IDs) > maxChartFoxBatch {
		writeErr(w, http.StatusBadRequest, errors.New("too many charts at once"))
		return
	}
	for _, id := range body.IDs {
		if !chartIDRe.MatchString(id) {
			writeErr(w, http.StatusBadRequest, errors.New("unknown chart"))
			return
		}
	}
	results := make([]chartfoxResult, 0, len(body.IDs))
	for _, id := range body.IDs {
		res := s.chartfoxImportOne(r.Context(), cf, id)
		results = append(results, res)
		if res.Status == "error" && res.Message == errNotConnected {
			break
		}
	}
	writeJSON(w, 200, map[string]any{"results": results})
}

func (s *Server) chartfoxImportOne(ctx context.Context, cf *chartfoxState, id string) chartfoxResult {
	file := chartfoxFile(id)
	if c, ok := s.lib.FindByOriginal(file); ok {
		return chartfoxResult{ID: id, Status: "already", ChartID: c.ID}
	}
	fail := func(err error) chartfoxResult {
		msg := err.Error()
		if errors.Is(err, chartfox.ErrUnauthorized) {
			msg = errNotConnected
		}
		return chartfoxResult{ID: id, Status: "error", Message: msg}
	}
	d, err := cf.catalog.Chart(ctx, id)
	if err != nil {
		return fail(err)
	}
	if len(d.Files) == 0 || d.Files[0].URL == "" {
		return fail(errors.New("ChartFox has no file for this chart"))
	}
	data, err := cf.catalog.Download(ctx, d.Files[0].URL)
	if err != nil {
		return fail(err)
	}
	typ := chartfox.Kind(d.TypeKey)
	title := strings.TrimSpace(d.Name)
	c, err := s.importChart(data, file, importForm{
		Source: chartfoxSource, ICAO: d.AirportICAO, Type: typ, Title: title,
		Name: naming.Display(strings.ToUpper(d.AirportICAO), typ, title),
	})
	var already errAlready
	switch {
	case errors.As(err, &already):
		return chartfoxResult{ID: id, Status: "already", ChartID: already.ID}
	case err != nil:
		return fail(err)
	}
	return chartfoxResult{ID: id, Status: "ok", ChartID: c.ID}
}
