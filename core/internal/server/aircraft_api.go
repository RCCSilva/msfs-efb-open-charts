package server

import (
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"

	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/navdb"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/perf"
)

// The aircraft profiles: the performance of the aircraft the user flies, read from the files of
// Little Navmap or written by hand, and used to estimate the time and the fuel of a route. What a
// profile is and how a flight is estimated is in package perf; this is its HTTP face.

func profileStatus(err error) int {
	switch {
	case errors.Is(err, perf.ErrInvalid), errors.Is(err, navdb.ErrInvalid):
		return 400
	case errors.Is(err, navdb.ErrNotFound):
		return 404
	}
	return 500
}

// profileOf reads a stored record as a profile, with the id and the name of the record.
func profileOf(rec navdb.AircraftProfile) (perf.Profile, error) {
	var p perf.Profile
	if err := json.Unmarshal(rec.Data, &p); err != nil {
		return p, err
	}
	p.ID, p.Name = rec.ID, rec.Name
	return p, nil
}

func recordOf(p perf.Profile) (navdb.AircraftProfile, error) {
	if err := p.Clean(); err != nil {
		return navdb.AircraftProfile{}, err
	}
	data, err := json.Marshal(p)
	return navdb.AircraftProfile{ID: p.ID, Name: p.Name, Data: data}, err
}

// GET /api/aircraft
func (s *Server) handleAircraftList(w http.ResponseWriter, r *http.Request) {
	db := s.navDB()
	if db == nil {
		navUnavailable(w)
		return
	}
	recs, err := db.AircraftProfiles()
	if err != nil {
		writeErr(w, 500, err)
		return
	}
	list := []perf.Profile{}
	for _, rec := range recs {
		if p, err := profileOf(rec); err == nil {
			list = append(list, p)
		}
	}
	writeJSON(w, 200, map[string]any{"aircraft": list})
}

// POST /api/aircraft/import: the body is a performance file of Little Navmap, as it is on disk.
func (s *Server) handleAircraftImport(w http.ResponseWriter, r *http.Request) {
	db := s.navDB()
	if db == nil {
		navUnavailable(w)
		return
	}
	data, err := io.ReadAll(io.LimitReader(r.Body, 256<<10))
	if err != nil {
		writeErr(w, 400, err)
		return
	}
	p, err := perf.Parse(data)
	if err != nil {
		writeErr(w, profileStatus(err), err)
		return
	}
	s.saveProfile(w, p, func(rec navdb.AircraftProfile) (navdb.AircraftProfile, error) { return db.CreateAircraftProfile(rec) })
}

// POST /api/aircraft and PUT /api/aircraft/{id}: the profile as the manager edits it.
func (s *Server) handleAircraftSave(w http.ResponseWriter, r *http.Request) {
	db := s.navDB()
	if db == nil {
		navUnavailable(w)
		return
	}
	var p perf.Profile
	if err := readJSON(r, &p); err != nil {
		writeErr(w, 400, err)
		return
	}
	save := func(rec navdb.AircraftProfile) (navdb.AircraftProfile, error) { return db.CreateAircraftProfile(rec) }
	if idText := r.PathValue("id"); idText != "" {
		id, err := strconv.ParseInt(idText, 10, 64)
		if err != nil {
			writeErr(w, 400, err)
			return
		}
		p.ID = id
		save = func(rec navdb.AircraftProfile) (navdb.AircraftProfile, error) { return db.UpdateAircraftProfile(rec) }
	}
	s.saveProfile(w, p, save)
}

func (s *Server) saveProfile(w http.ResponseWriter, p perf.Profile, save func(navdb.AircraftProfile) (navdb.AircraftProfile, error)) {
	rec, err := recordOf(p)
	if err == nil {
		rec, err = save(rec)
	}
	if err != nil {
		writeErr(w, profileStatus(err), err)
		return
	}
	out, err := profileOf(rec)
	if err != nil {
		writeErr(w, 500, err)
		return
	}
	writeJSON(w, 200, out)
}

// DELETE /api/aircraft/{id}
func (s *Server) handleAircraftDelete(w http.ResponseWriter, r *http.Request) {
	db := s.navDB()
	if db == nil {
		navUnavailable(w)
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeErr(w, 400, err)
		return
	}
	if err := db.DeleteAircraftProfile(id); err != nil {
		writeErr(w, profileStatus(err), err)
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

// POST /api/route/estimate   {"profileId": 3, "legsNm": [0, 80, 120], "cruiseFt": 8000, ...}
// The fields after cruiseFt are those of perf.Flight and are optional.
func (s *Server) handleRouteEstimate(w http.ResponseWriter, r *http.Request) {
	db := s.navDB()
	if db == nil {
		navUnavailable(w)
		return
	}
	var body struct {
		ProfileID     int64     `json:"profileId"`
		LegsNm        []float64 `json:"legsNm"`
		CruiseFt      float64   `json:"cruiseFt"`
		DepartureFt   float64   `json:"departureFt"`
		DestinationFt float64   `json:"destinationFt"`
		// Departure and Destination are airports (ICAO): their elevation, from the navigation database,
		// is used when the elevation itself is not given. An airport that is not known counts as sea level.
		Departure   string  `json:"departure"`
		Destination string  `json:"destination"`
		AlternateNm float64 `json:"alternateNm"`
		WindKt        float64   `json:"windKt"`
	}
	if err := readJSON(r, &body); err != nil {
		writeErr(w, 400, err)
		return
	}
	if len(body.LegsNm) > 500 {
		writeErr(w, 400, errors.New("the route has too many legs"))
		return
	}
	rec, err := db.AircraftProfile(body.ProfileID)
	if err != nil {
		writeErr(w, profileStatus(err), err)
		return
	}
	p, err := profileOf(rec)
	if err != nil {
		writeErr(w, 500, err)
		return
	}
	elevation := func(icao string, given float64) float64 {
		if given != 0 || strings.TrimSpace(icao) == "" {
			return given
		}
		if a, err := db.Airport(icao); err == nil {
			return math.Max(a.AltitudeM/0.3048, 0)
		}
		return 0
	}
	body.DepartureFt = elevation(body.Departure, body.DepartureFt)
	body.DestinationFt = elevation(body.Destination, body.DestinationFt)
	res, err := p.Estimate(perf.Flight{LegsNm: body.LegsNm, CruiseFt: body.CruiseFt, DepartureFt: body.DepartureFt,
		DestinationFt: body.DestinationFt, AlternateNm: body.AlternateNm, WindKt: body.WindKt})
	if err != nil {
		writeErr(w, profileStatus(err), err)
		return
	}
	writeJSON(w, 200, res)
}
