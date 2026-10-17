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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/dbadmin"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/navdata"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/navdb"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/settings"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/sim"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/store"
)

// dbServer is a server wired the way the program's start-up wires it: the database is opened by a
// function that also tells the server why it could not be used, and Replace closes it, lets the
// change happen, and opens it again. The files are real: what the maintenance does to them is what
// these tests look at (the steps themselves are tested in package dbadmin, with mocks).
type dbServer struct {
	*Server
	path string
	cur  *navdb.DB
}

func newDBServer(t *testing.T, before func(path string)) *dbServer {
	t.Helper()
	lib, err := store.Open(t.TempDir())
	require.NoError(t, err)
	dir := t.TempDir()
	path := filepath.Join(dir, "navdata.db")
	if before != nil {
		before(path)
	}
	s := New(lib, nil, sim.Provider{}, "127.0.0.1:43214")
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
		s.SetNav(nav)
		s.SetNavProblem("", "")
		return nil
	}
	_ = open()
	s.SetNavFile(NavFile{Path: path, Replace: func(change func() error) error {
		s.SetNav(nil)
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

func (d *dbServer) status(t *testing.T) dbadmin.Status {
	t.Helper()
	w := httptest.NewRecorder()
	d.handleDBStatus(w, httptest.NewRequest("GET", "/api/db/status", nil))
	require.Equal(t, 200, w.Code)
	var out dbadmin.Status
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
	return out
}

func (d *dbServer) airports(t *testing.T) int {
	t.Helper()
	require.NotNil(t, d.cur, "the database is not open")
	c, err := d.cur.Counts()
	require.NoError(t, err)
	return c.Airports
}

func backupsOf(t *testing.T, path string) []navdb.BackupInfo {
	t.Helper()
	l, err := navdb.ListBackups(path)
	require.NoError(t, err)
	return l
}

func kindsOf(backups []navdb.BackupInfo) map[string]int {
	kinds := map[string]int{}
	for _, b := range backups {
		kinds[b.Kind]++
	}
	return kinds
}

func TestStatusOfAHealthyDatabase(t *testing.T) {
	// Arrange
	d := newDBServer(t, nil)

	// Act
	st := d.status(t)

	// Assert
	assert.True(t, st.OK)
	assert.Nil(t, st.Problem)
	assert.Equal(t, d.path, st.Path)
	require.NotNil(t, st.Schema)
	assert.Equal(t, navdb.LatestSchema(), *st.Schema)
}

func TestBackupMakesACopyInTheBackupsFolder(t *testing.T) {
	// Arrange
	d := newDBServer(t, nil)

	// Act
	w := post(d.handleDBBackup, ``)

	// Assert
	require.Equal(t, 200, w.Code, w.Body.String())
	assert.Equal(t, 1, kindsOf(backupsOf(t, d.path))["manual"])
}

func TestResetCopiesFirstAndStartsOver(t *testing.T) {
	// Arrange
	d := newDBServer(t, nil)
	require.NoError(t, d.cur.SaveAirport(&navdata.Airport{ICAO: "SBSP", Name: "Congonhas"}))

	// Act
	w := post(d.handleDBReset, ``)

	// Assert
	require.Equal(t, 200, w.Code, w.Body.String())
	assert.Zero(t, d.airports(t), "the database is new")
	assert.Equal(t, 1, kindsOf(backupsOf(t, d.path))["reset"], "a copy was made first")
}

func TestRestoringACopyBringsTheDataBack(t *testing.T) {
	// Arrange
	d := newDBServer(t, nil)
	require.NoError(t, d.cur.SaveAirport(&navdata.Airport{ICAO: "SBSP", Name: "Congonhas"}))
	require.Equal(t, 200, post(d.handleDBReset, ``).Code)
	require.Zero(t, d.airports(t))
	var name string
	for _, b := range backupsOf(t, d.path) {
		if b.Kind == "reset" {
			name = b.Name
		}
	}
	require.NotEmpty(t, name)

	// Act
	w := post(d.handleDBRestore, `{"name":"`+name+`"}`)

	// Assert
	require.Equal(t, 200, w.Code, w.Body.String())
	assert.Equal(t, 1, d.airports(t), "the airport is back")
	assert.Equal(t, 1, kindsOf(backupsOf(t, d.path))["restore"], "a copy of what was there was made first")
}

func uploadDatabase(t *testing.T, d *dbServer, content []byte) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("file", "x.db")
	require.NoError(t, err)
	_, err = fw.Write(content)
	require.NoError(t, err)
	require.NoError(t, mw.Close())
	req := httptest.NewRequest("POST", "/api/db/import", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	w := httptest.NewRecorder()
	d.handleDBImport(w, req)
	return w
}

func TestImportRefusesAFileThatIsNotADatabase(t *testing.T) {
	// Arrange
	d := newDBServer(t, nil)
	require.NoError(t, d.cur.SaveAirport(&navdata.Airport{ICAO: "SBSP"}))

	// Act
	w := uploadDatabase(t, d, []byte("this is not a database, only some text that is long enough to look like a file"))

	// Assert
	assert.Equal(t, 422, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "damaged")
	assert.Equal(t, 1, d.airports(t), "the current database is untouched")
	_, err := os.Stat(navdb.IncomingPath(d.path))
	assert.True(t, os.IsNotExist(err), "the refused file is not left behind")
}

func TestImportReplacesTheDatabaseWithAGoodFile(t *testing.T) {
	// Arrange
	d := newDBServer(t, nil)
	require.NoError(t, d.cur.SaveAirport(&navdata.Airport{ICAO: "SBSP"}))
	good := filepath.Join(t.TempDir(), "other.db")
	other, err := navdb.Open(good)
	require.NoError(t, err)
	for _, icao := range []string{"KJFK", "KLAX"} {
		require.NoError(t, other.SaveAirport(&navdata.Airport{ICAO: icao}))
	}
	other.Close()
	content, err := os.ReadFile(good)
	require.NoError(t, err)

	// Act
	w := uploadDatabase(t, d, content)

	// Assert
	require.Equal(t, 200, w.Code, w.Body.String())
	assert.Equal(t, 2, d.airports(t))
	assert.Equal(t, 1, kindsOf(backupsOf(t, d.path))["restore"], "what was there was copied first")
}

func TestImportWithoutAFileIsABadRequest(t *testing.T) {
	// Arrange
	d := newDBServer(t, nil)
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	require.NoError(t, mw.WriteField("other", "x"))
	require.NoError(t, mw.Close())
	req := httptest.NewRequest("POST", "/api/db/import", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	w := httptest.NewRecorder()

	// Act
	d.handleDBImport(w, req)

	// Assert
	assert.Equal(t, 400, w.Code)
	assert.Contains(t, w.Body.String(), "no file")
}

// A damaged file does not stop the program: it says why, and starting over sets the file aside.
func TestADamagedFileIsReportedAndSetAside(t *testing.T) {
	// Arrange
	d := newDBServer(t, func(path string) {
		require.NoError(t, os.WriteFile(path, []byte("this is not a database, only some text that is long enough to look like a file"), 0o644))
	})
	before := d.status(t)

	// Act
	reset := post(d.handleDBReset, ``)

	// Assert
	assert.False(t, before.OK)
	require.NotNil(t, before.Problem)
	assert.Equal(t, "corrupt", before.Problem.Kind)
	require.Equal(t, 200, reset.Code, reset.Body.String())
	after := d.status(t)
	assert.True(t, after.OK)
	assert.Nil(t, after.Problem)
	assert.Equal(t, 1, kindsOf(backupsOf(t, d.path))["corrupt"], "the damaged file is kept")
}

func TestNothingWorksWithoutTheDatabase(t *testing.T) {
	// Arrange
	d := newDBServer(t, func(path string) {
		require.NoError(t, os.WriteFile(path, []byte("this is not a database, only some text that is long enough to look like a file"), 0o644))
	})

	// Act
	save := post(d.handleRouteSave, `{"text":"SBSP SBBR"}`)
	backup := post(d.handleDBBackup, ``)

	// Assert
	assert.Equal(t, 503, save.Code)
	assert.Equal(t, 409, backup.Code, "there is nothing to copy")
}

// A file made by a newer version is reported and left as it is.
func TestANewerFileIsReportedAndNotChanged(t *testing.T) {
	// Arrange
	d := newDBServer(t, func(path string) {
		db, err := navdb.Open(path)
		require.NoError(t, err)
		db.Close()
		raw, err := sql.Open("sqlite", "file:"+path)
		require.NoError(t, err)
		defer raw.Close()
		_, err = raw.Exec(fmt.Sprintf(`PRAGMA user_version = %d`, navdb.LatestSchema()+3))
		require.NoError(t, err)
	})

	// Act
	st := d.status(t)

	// Assert
	assert.False(t, st.OK)
	require.NotNil(t, st.Problem)
	assert.Equal(t, "newer", st.Problem.Kind)
	raw, err := sql.Open("sqlite", "file:"+d.path)
	require.NoError(t, err)
	defer raw.Close()
	var v int
	require.NoError(t, raw.QueryRow(`PRAGMA user_version`).Scan(&v))
	assert.Equal(t, navdb.LatestSchema()+3, v, "the file was not touched")
}
