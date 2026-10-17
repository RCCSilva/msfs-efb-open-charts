package server

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/navdata"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/navdb"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/settings"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/store"
)

// dbServer is a server wired the way the program's start-up wires it: the database is opened by a
// function that also tells the server why it could not be used, and Replace closes it, lets the
// change happen, and opens it again.
type dbServer struct {
	*Server
	path string
	cur  *navdb.DB
}

func newDBServer(t *testing.T, before func(path string)) *dbServer {
	t.Helper()
	lib, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "navdata.db")
	if before != nil {
		before(path)
	}
	s := New(lib, nil, func() Position { return Position{} }, "127.0.0.1:43214")
	s.UseSettings(dir, dir, settings.Settings{}, true)
	d := &dbServer{Server: s, path: path}
	open := func() error {
		nav, err := navdb.Open(path)
		if err != nil {
			kind := "failed"
			switch {
			case errors.Is(err, navdb.ErrCorrupt):
				kind = "corrupt"
			case errors.Is(err, navdb.ErrNewerSchema):
				kind = "newer"
			}
			s.SetNavProblem(kind, err.Error())
			return err
		}
		d.cur = nav
		s.SetNav(nav, nil, nil, nil)
		s.SetNavProblem("", "")
		return nil
	}
	_ = open()
	s.SetNavFile(NavFile{Path: path, Replace: func(change func() error) error {
		s.SetNav(nil, nil, nil, nil)
		if d.cur != nil {
			d.cur.Close()
			d.cur = nil
		}
		cerr := change()
		oerr := open()
		if cerr != nil {
			return cerr
		}
		return oerr
	}})
	t.Cleanup(func() {
		if d.cur != nil {
			d.cur.Close()
		}
	})
	return d
}

func (d *dbServer) status(t *testing.T) map[string]any {
	t.Helper()
	w := httptest.NewRecorder()
	d.handleDBStatus(w, httptest.NewRequest("GET", "/api/db/status", nil))
	var out map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func (d *dbServer) airports(t *testing.T) int {
	t.Helper()
	if d.cur == nil {
		t.Fatal("the database is not open")
	}
	c, err := d.cur.Counts()
	if err != nil {
		t.Fatal(err)
	}
	return c.Airports
}

func backupsOf(t *testing.T, path string) []navdb.BackupInfo {
	t.Helper()
	l, err := navdb.ListBackups(path)
	if err != nil {
		t.Fatal(err)
	}
	return l
}

func TestDBResetBackupsFirstAndStartsOver(t *testing.T) {
	d := newDBServer(t, nil)
	if err := d.cur.SaveAirport(&navdata.Airport{ICAO: "SBSP", Name: "Congonhas"}); err != nil {
		t.Fatal(err)
	}
	if st := d.status(t); st["ok"] != true || st["problem"] != nil {
		t.Fatalf("%v", st)
	}

	// a copy the user asks for
	if w := post(d.handleDBBackup, ``); w.Code != 200 {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	// starting over: a copy first, then a new, empty database
	if w := post(d.handleDBReset, ``); w.Code != 200 {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	if d.airports(t) != 0 {
		t.Fatal("the database is new")
	}
	kinds := map[string]int{}
	for _, b := range backupsOf(t, d.path) {
		kinds[b.Kind]++
	}
	if kinds["manual"] != 1 || kinds["reset"] != 1 {
		t.Fatalf("%v", kinds)
	}

	// restoring the copy brings the airport back (after a copy of what was there)
	var name string
	for _, b := range backupsOf(t, d.path) {
		if b.Kind == "reset" {
			name = b.Name
		}
	}
	if w := post(d.handleDBRestore, `{"name":"`+name+`"}`); w.Code != 200 {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	if d.airports(t) != 1 {
		t.Fatal("the airport is back")
	}
	if got := backupsOf(t, d.path); len(got) != 3 {
		t.Fatalf("manual, reset and the copy made before restoring: %+v", got)
	}
}

func TestDBRestoreRefusesWhatIsNotAHealthyNavigationDatabase(t *testing.T) {
	d := newDBServer(t, nil)
	if err := d.cur.SaveAirport(&navdata.Airport{ICAO: "SBSP"}); err != nil {
		t.Fatal(err)
	}
	upload := func(content []byte) *httptest.ResponseRecorder {
		var buf bytes.Buffer
		mw := multipart.NewWriter(&buf)
		fw, _ := mw.CreateFormFile("file", "x.db")
		_, _ = fw.Write(content)
		_ = mw.Close()
		req := httptest.NewRequest("POST", "/api/db/import", &buf)
		req.Header.Set("Content-Type", mw.FormDataContentType())
		w := httptest.NewRecorder()
		d.handleDBImport(w, req)
		return w
	}
	w := upload([]byte("this is not a database, only some text that is long enough to look like a file"))
	if w.Code != 422 || !strings.Contains(w.Body.String(), "damaged") {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	if d.airports(t) != 1 {
		t.Fatal("the current database is untouched")
	}
	if _, err := os.Stat(navdb.IncomingPath(d.path)); err == nil {
		t.Fatal("the refused file is not left behind")
	}

	// a good file takes the place of the database
	good := filepath.Join(t.TempDir(), "other.db")
	o, err := navdb.Open(good)
	if err != nil {
		t.Fatal(err)
	}
	for _, icao := range []string{"KJFK", "KLAX"} {
		if err := o.SaveAirport(&navdata.Airport{ICAO: icao}); err != nil {
			t.Fatal(err)
		}
	}
	o.Close()
	content, _ := os.ReadFile(good)
	if w := upload(content); w.Code != 200 {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	if d.airports(t) != 2 {
		t.Fatalf("%d airports", d.airports(t))
	}
	// and what was there was copied first
	n := 0
	for _, b := range backupsOf(t, d.path) {
		if b.Kind == "restore" {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("%d copies before restoring", n)
	}
}

// A damaged file does not stop the program: it says why, and starting over sets the file aside.
func TestDBDamagedFileIsReportedAndSetAside(t *testing.T) {
	d := newDBServer(t, func(path string) {
		_ = os.WriteFile(path, []byte("this is not a database, only some text that is long enough to look like a file"), 0o644)
	})
	st := d.status(t)
	p, _ := st["problem"].(map[string]any)
	if st["ok"] != false || p["kind"] != "corrupt" {
		t.Fatalf("%v", st)
	}
	if w := post(d.handleRouteSave, `{"text":"SBSP SBBR"}`); w.Code == 200 {
		t.Fatal("nothing works without the database")
	}
	if w := post(d.handleDBBackup, ``); w.Code != 409 {
		t.Fatalf("there is nothing to copy: %d", w.Code)
	}
	if w := post(d.handleDBReset, ``); w.Code != 200 {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	if st := d.status(t); st["ok"] != true || st["problem"] != nil {
		t.Fatalf("%v", st)
	}
	var damaged int
	for _, b := range backupsOf(t, d.path) {
		if b.Kind == "corrupt" {
			damaged++
		}
	}
	if damaged != 1 {
		t.Fatalf("the damaged file is kept: %d", damaged)
	}
}

// A file made by a newer version is reported and left as it is.
func TestDBNewerFileIsReported(t *testing.T) {
	d := newDBServer(t, func(path string) {
		db, err := navdb.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		db.Close()
		raw, err := sql.Open("sqlite", "file:"+path)
		if err != nil {
			t.Fatal(err)
		}
		defer raw.Close()
		if _, err := raw.Exec(fmt.Sprintf(`PRAGMA user_version = %d`, navdb.LatestSchema()+3)); err != nil {
			t.Fatal(err)
		}
	})
	st := d.status(t)
	p, _ := st["problem"].(map[string]any)
	if st["ok"] != false || p["kind"] != "newer" {
		t.Fatalf("%v", st)
	}
	raw, err := sql.Open("sqlite", "file:"+d.path)
	if err != nil {
		t.Fatal(err)
	}
	defer raw.Close()
	var v int
	if err := raw.QueryRow(`PRAGMA user_version`).Scan(&v); err != nil || v != navdb.LatestSchema()+3 {
		t.Fatalf("the file was not touched: %d %v", v, err)
	}
}
