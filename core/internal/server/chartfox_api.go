package server

import (
	"net/http"

	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/chartfox"
)

// UseChartFox gives the server the user's ChartFox account. Without it the ChartFox endpoints answer that the
// account is not available.
func (s *Server) UseChartFox(c chartfox.Connector) { s.chartfox = c }

func (s *Server) handleChartFoxStatus(w http.ResponseWriter, r *http.Request) {
	if s.chartfox == nil {
		writeJSON(w, 200, chartfox.Status{})
		return
	}
	writeJSON(w, 200, s.chartfox.Status())
}

// handleChartFoxConnect starts a login: the authorization page opens in the user's browser and the answer comes back to
// the loopback listener of the login, not to this server. The status endpoint tells how it went.
func (s *Server) handleChartFoxConnect(w http.ResponseWriter, r *http.Request) {
	if s.chartfox == nil {
		http.Error(w, "ChartFox is not available", http.StatusNotFound)
		return
	}
	if err := s.chartfox.Connect(); err != nil {
		writeErr(w, http.StatusConflict, err)
		return
	}
	writeJSON(w, http.StatusAccepted, s.chartfox.Status())
}

func (s *Server) handleChartFoxDisconnect(w http.ResponseWriter, r *http.Request) {
	if s.chartfox == nil {
		http.Error(w, "ChartFox is not available", http.StatusNotFound)
		return
	}
	if err := s.chartfox.Disconnect(); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, 200, s.chartfox.Status())
}
