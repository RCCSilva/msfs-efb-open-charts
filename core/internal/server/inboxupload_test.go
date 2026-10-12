package server

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestInboxNameIsSafe(t *testing.T) {
	cases := map[string]string{
		"sbsp_adc_adc_20261001.pdf":           "sbsp_adc_adc_20261001.pdf",
		"../../evil.pdf":                      "evil.pdf",
		`C:\Users\x\pacote-full-20261001.zip`: "pacote-full-20261001.zip",
		"a:b*c?.pdf":                          "a_b_c_.pdf",
		"notes.txt":                           "",
		"..pdf":                               "",
		".zip":                                "",
		"":                                    "",
	}
	for in, want := range cases {
		if got := inboxName(in); got != want {
			t.Errorf("inboxName(%q) = %q, want %q", in, got, want)
		}
	}
}

func upload(t *testing.T, s *Server, name string, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("file", name)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = fw.Write(body)
	_ = mw.Close()
	req := httptest.NewRequest("POST", "/api/inbox/upload", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.RemoteAddr = "127.0.0.1:50000"
	req.Host = "127.0.0.1:43210"
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	return rec
}

func TestInboxUploadKeepsPdfAndZipInTheInbox(t *testing.T) {
	s, _ := newAccessServer(t, false)

	rec := upload(t, s, "pacote-full-20261001.zip", []byte("PK\x03\x04 not a real zip"))
	if rec.Code != 201 {
		t.Fatalf("zip upload: %d %s", rec.Code, rec.Body.String())
	}
	var got struct {
		Name string `json:"name"`
		Path string `json:"path"`
		Kind string `json:"kind"`
		Size int64  `json:"size"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Name != "pacote-full-20261001.zip" || got.Kind != "zip" || got.Size == 0 {
		t.Fatalf("answer = %+v", got)
	}
	if filepath.Dir(got.Path) != s.inboxDir() {
		t.Fatalf("saved outside the inbox: %s", got.Path)
	}
	if _, err := os.Stat(got.Path); err != nil {
		t.Fatalf("file missing: %v", err)
	}

	// a path in the name must not escape the inbox
	rec = upload(t, s, "../../escape.pdf", []byte("%PDF-1.4"))
	if rec.Code != 201 {
		t.Fatalf("pdf upload: %d", rec.Code)
	}
	if _, err := os.Stat(filepath.Join(s.inboxDir(), "escape.pdf")); err != nil {
		t.Fatalf("the base name should have been kept in the inbox: %v", err)
	}

	// other types are refused
	if rec = upload(t, s, "script.exe", []byte("MZ")); rec.Code != 400 {
		t.Fatalf("an .exe must be refused, got %d", rec.Code)
	}
}
