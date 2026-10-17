package efbapp

import "github.com/RCCSilva/msfs-efb-open-charts/core/internal/sim"

// Installer is the app installer of MSFS 2024, as the program's sim.AppInstaller.
type Installer struct{}

func (Installer) Detect(exePath string) []sim.AppInstallation {
	var out []sim.AppInstallation
	for _, in := range Detect(exePath) {
		out = append(out, sim.AppInstallation{
			Edition: in.Edition, PackagesPath: in.PackagesPath, Target: in.Community, AutoStartFile: in.ExeXML,
			App: appState(in.EFB), AutoStart: in.AutoStart,
		})
	}
	return out
}

func (Installer) ValidTarget(target string) error { return ValidCommunity(target) }

func (Installer) Install(target, serverURL string) error { return InstallEFB(target, serverURL) }

func (Installer) Remove(target string) error { return RemoveEFB(target) }

func (Installer) State(target string) sim.AppState { return appState(EFBState(target)) }

func (Installer) InstalledServerURL(target string) string { return InstalledServerURL(target) }

func (Installer) AutoStart(file string) bool { return AutoStartEnabled(file) }

func (Installer) SetAutoStart(file, exePath string, args []string, enable bool) error {
	return SetAutoStart(file, exePath, args, enable)
}

func appState(e EFB) sim.AppState {
	return sim.AppState{
		Installed: e.Installed, Legacy: e.Legacy, ServerURL: e.ServerURL, URLMismatch: e.URLMismatch,
		Version: e.Version, Build: e.Build, UpToDate: e.UpToDate, Path: e.Path,
	}
}
