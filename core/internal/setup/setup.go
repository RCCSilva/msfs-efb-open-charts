// Package setup finds the user's Microsoft Flight Simulator installation and, only when the user
// asks, installs the EFB app into its Community folder and registers Open Charts to start with
// the simulator. Nothing here runs by itself.
package setup

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/buildinfo"
)

//go:embed efbpkg/*
var efbFiles embed.FS

// EFBPackageName is the folder created in Community. It stays the same across versions so an
// update replaces the files in place.
const EFBPackageName = "open-charts-efb"

// legacyPackageName is the folder used before the app was renamed; it is removed on install.
const legacyPackageName = "local-moving-map-efb"

// EFBVersion is written to the package manifest; bump it when the embedded app changes.
const EFBVersion = "0.4.0"

const efbAppDir = "html_ui/efb_ui/efb_apps/OpenCharts"

// Install is one detected simulator installation.
type Install struct {
	Edition      string `json:"edition"` // "steam" | "store"
	PackagesPath string `json:"packagesPath"`
	Community    string `json:"community"`
	ExeXML       string `json:"exeXml"`
	EFB          EFB    `json:"efb"`
	AutoStart    bool   `json:"autoStart"`
}

// EFB describes the state of the EFB app in a Community folder.
type EFB struct {
	Installed bool `json:"installed"`
	Legacy    bool `json:"legacy"` // the old "Local Moving Map" package is present
	// ServerURL is the address written into the installed app. If it differs from where the
	// server really is (the port changed), the app cannot reach it until it is updated.
	ServerURL   string `json:"serverURL,omitempty"`
	URLMismatch bool   `json:"urlMismatch"`
	Version     string `json:"version,omitempty"`
	Build       string `json:"build,omitempty"` // what the installed app says it is: "0.1.1" or "0.1.1-8fa9b5b"
	UpToDate    bool   `json:"upToDate"`
	Path        string `json:"path"`
}

var installedPathRe = regexp.MustCompile(`(?m)^\s*InstalledPackagesPath\s+"([^"]+)"`)

// candidateRoots are the folders that hold UserCfg.opt (and where EXE.xml lives).
func candidateRoots() []struct{ edition, dir string } {
	var out []struct{ edition, dir string }
	if a := os.Getenv("APPDATA"); a != "" {
		out = append(out, struct{ edition, dir string }{"steam", filepath.Join(a, "Microsoft Flight Simulator 2024")})
	}
	if l := os.Getenv("LOCALAPPDATA"); l != "" {
		out = append(out, struct{ edition, dir string }{"store",
			filepath.Join(l, "Packages", "Microsoft.Limitless_8wekyb3d8bbwe", "LocalCache")})
	}
	return out
}

// Detect lists the installations whose configuration can be found.
func Detect(exePath string) []Install {
	var res []Install
	for _, r := range candidateRoots() {
		b, err := os.ReadFile(filepath.Join(r.dir, "UserCfg.opt"))
		if err != nil {
			continue
		}
		m := installedPathRe.FindSubmatch(b)
		if m == nil {
			continue
		}
		in := Install{Edition: r.edition, PackagesPath: string(m[1]), ExeXML: filepath.Join(r.dir, "EXE.xml")}
		in.Community = filepath.Join(in.PackagesPath, "Community")
		in.EFB = EFBState(in.Community)
		in.AutoStart = AutoStartEnabled(in.ExeXML)
		res = append(res, in)
	}
	return res
}

// ---------------------------------------------------------------- EFB app

// EFBState reports whether the app is in the Community folder and whether it is current.
func EFBState(community string) EFB {
	dir := filepath.Join(community, EFBPackageName)
	st := EFB{Path: dir}
	if _, err := os.Stat(filepath.Join(community, legacyPackageName)); err == nil {
		st.Legacy = true
	}
	b, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		return st
	}
	st.Installed = true
	st.ServerURL = readServerURL(dir)
	var m struct {
		Version string `json:"package_version"`
	}
	_ = json.Unmarshal(b, &m)
	st.Version = m.Version
	st.Build = readBuild(dir)
	// The manifest holds the package version; the build also has to match, so a development
	// build (0.1.1-8fa9b5b) is told apart from the one installed before it.
	st.UpToDate = m.Version == EFBVersion && st.Build == buildinfo.Display(EFBVersion)
	return st
}

// ValidCommunity checks that a path is an existing Community folder.
func ValidCommunity(p string) error {
	if !strings.HasPrefix(strings.ToLower(filepath.Base(p)), "community") {
		return errors.New("not a Community folder")
	}
	if st, err := os.Stat(p); err != nil || !st.IsDir() {
		return errors.New("folder does not exist")
	}
	return nil
}

// InstallEFB writes the app into community/local-moving-map-efb. serverURL (e.g.
// "http://127.0.0.1:43210/") is written into the app's configuration.
func InstallEFB(community, serverURL string) error {
	if err := ValidCommunity(community); err != nil {
		return err
	}
	if !strings.HasSuffix(serverURL, "/") {
		serverURL += "/"
	}
	js, err := efbFiles.ReadFile("efbpkg/OpenCharts.js")
	if err != nil {
		return err
	}
	src := string(js)
	re := regexp.MustCompile(`var SERVER_URL = '[^']*';`)
	if !re.MatchString(src) {
		return errors.New("embedded app has no SERVER_URL")
	}
	src = re.ReplaceAllLiteralString(src, "var SERVER_URL = '"+serverURL+"';")
	src = efbVersionRe.ReplaceAllLiteralString(src, "var VERSION = '"+buildinfo.Display(EFBVersion)+"';")
	icon, _ := efbFiles.ReadFile("efbpkg/app-icon.svg")
	thumb, _ := efbFiles.ReadFile("efbpkg/Thumbnail.jpg")

	dir := filepath.Join(community, EFBPackageName)
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	// Two apps with different internal names would both show up in the EFB: drop the old one.
	if err := os.RemoveAll(filepath.Join(community, legacyPackageName)); err != nil {
		return err
	}
	type file struct {
		path string
		data []byte
	}
	files := []file{
		{"ContentInfo/" + EFBPackageName + "/Thumbnail.jpg", thumb},
		{efbAppDir + "/Assets/app-icon.svg", icon},
		{efbAppDir + "/OpenCharts.js", []byte(src)},
	}
	// The simulator's own build writes a layout.json with lowercase paths, sizes and a
	// Windows FILETIME per file; the same is done here.
	type entry struct {
		Path string `json:"path"`
		Size int    `json:"size"`
		Date int64  `json:"date"`
	}
	ft := time.Now().UTC().UnixNano()/100 + 116444736000000000
	var layout struct {
		Content []entry `json:"content"`
	}
	total := 0
	for _, f := range files {
		full := filepath.Join(dir, filepath.FromSlash(f.path))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(full, f.data, 0o644); err != nil {
			return err
		}
		layout.Content = append(layout.Content, entry{strings.ToLower(f.path), len(f.data), ft})
		total += len(f.data)
	}
	manifest := map[string]any{
		"dependencies": []any{}, "content_type": "MISC", "title": "Open Charts", "manufacturer": "",
		"creator": "Rafael Coelho", "package_version": EFBVersion, "minimum_game_version": "1.8.16",
		"minimum_compatibility_version": "8.11.0.236", "export_type": "Community",
		"builder": "Microsoft Flight Simulator 2024", "package_order_hint": "MISC",
		"release_notes":      map[string]any{"neutral": map[string]string{"LastUpdate": "", "OlderHistory": ""}},
		"total_package_size": fmt.Sprint(total),
	}
	for name, v := range map[string]any{"layout.json": layout, "manifest.json": manifest} {
		b, _ := json.MarshalIndent(v, "", "  ")
		if err := os.WriteFile(filepath.Join(dir, name), b, 0o644); err != nil {
			return err
		}
	}
	return nil
}

// RemoveEFB deletes the app folder from Community.
func RemoveEFB(community string) error {
	if err := ValidCommunity(community); err != nil {
		return err
	}
	_ = os.RemoveAll(filepath.Join(community, legacyPackageName))
	return os.RemoveAll(filepath.Join(community, EFBPackageName))
}

// ---------------------------------------------------------------- start with the simulator

const addonName = "Open Charts"

var addonBlockRe = regexp.MustCompile(`(?s)[ \t]*<Launch\.Addon>\s*<Name>` + regexp.QuoteMeta(addonName) + `</Name>.*?</Launch\.Addon>[ \t]*\r?\n?`)

// AutoStartEnabled reports whether EXE.xml contains the Open Charts entry.
func AutoStartEnabled(exeXML string) bool {
	b, err := os.ReadFile(exeXML)
	if err != nil {
		return false
	}
	return addonBlockRe.Match(b)
}

func nonASCII(s string) bool {
	for _, r := range s {
		if r > 127 {
			return true
		}
	}
	return false
}

func xmlEscape(s string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;").Replace(s)
}

// SetAutoStart adds or removes the Open Charts entry in EXE.xml, keeping every other addon.
// The first time, the original file is copied to EXE.xml.opencharts.bak.
func SetAutoStart(exeXML, exePath string, args []string, enable bool) error {
	cur, err := os.ReadFile(exeXML)
	exists := err == nil
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if enable && (nonASCII(exePath) || nonASCII(strings.Join(args, " "))) {
		return errors.New("the program path has non-ASCII characters, which EXE.xml cannot hold safely: move Open Charts to a plain path")
	}
	text := string(cur)
	if exists {
		bak := exeXML + ".opencharts.bak"
		if _, err := os.Stat(bak); errors.Is(err, os.ErrNotExist) {
			_ = os.WriteFile(bak, cur, 0o644)
		}
	}
	text = addonBlockRe.ReplaceAllString(text, "")
	if enable {
		block := "\t<Launch.Addon>\r\n" +
			"\t\t<Name>" + addonName + "</Name>\r\n" +
			"\t\t<Disabled>False</Disabled>\r\n" +
			"\t\t<Path>" + xmlEscape(exePath) + "</Path>\r\n" +
			"\t\t<CommandLine>" + xmlEscape(strings.Join(args, " ")) + "</CommandLine>\r\n" +
			"\t\t<NewConsole>False</NewConsole>\r\n" +
			"\t</Launch.Addon>\r\n"
		if !exists || !strings.Contains(text, "</SimBase.Document>") {
			text = "<?xml version=\"1.0\" encoding=\"Windows-1252\"?>\r\n" +
				"<SimBase.Document Type=\"Launch\" version=\"1,0\">\r\n" +
				"\t<Descr>Launch</Descr>\r\n\t<Filename>EXE.xml</Filename>\r\n" +
				"\t<Disabled>False</Disabled>\r\n\t<Launch.ManualLoad>False</Launch.ManualLoad>\r\n" +
				block + "</SimBase.Document>\r\n"
		} else {
			i := strings.LastIndex(text, "</SimBase.Document>")
			text = text[:i] + block + text[i:]
		}
	}
	tmp := exeXML + ".tmp"
	if err := os.MkdirAll(filepath.Dir(exeXML), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(tmp, []byte(text), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, exeXML)
}

var efbVersionRe = regexp.MustCompile(`var VERSION = '[^']*';`)

// readBuild returns the build written into the installed EFB app ("0.1.1-8fa9b5b"), "" if unreadable.
func readBuild(packageDir string) string {
	b, err := os.ReadFile(filepath.Join(packageDir, filepath.FromSlash(efbAppDir), "OpenCharts.js"))
	if err != nil {
		return ""
	}
	if m := efbVersionRe.FindSubmatch(b); m != nil {
		v := string(m[0])
		v = strings.TrimPrefix(v, "var VERSION = '")
		return strings.TrimSuffix(v, "';")
	}
	return ""
}

var serverURLRe = regexp.MustCompile(`var SERVER_URL = '([^']*)';`)

// InstalledServerURL returns the address written into the EFB app installed in a Community
// folder, whether it is the current package or the one of the earlier "Local Moving Map" name.
func InstalledServerURL(community string) string {
	if u := readServerURL(filepath.Join(community, EFBPackageName)); u != "" {
		return u
	}
	return readServerURLFile(filepath.Join(community, legacyPackageName,
		"html_ui", "efb_ui", "efb_apps", "LocalMovingMap", "LocalMovingMap.js"))
}

// readServerURL returns the SERVER_URL written into the installed EFB app ("" if unreadable).
func readServerURL(packageDir string) string {
	return readServerURLFile(filepath.Join(packageDir, filepath.FromSlash(efbAppDir), "OpenCharts.js"))
}

func readServerURLFile(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	if m := serverURLRe.FindSubmatch(b); m != nil {
		return string(m[1])
	}
	return ""
}
