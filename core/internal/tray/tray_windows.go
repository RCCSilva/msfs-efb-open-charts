//go:build windows

// Package tray puts Open Charts in the Windows notification area: click to open the manager,
// right-click for the menu (status, open, quit).
package tray

import (
	"os/exec"
	"time"

	"fyne.io/systray"
)

// Config describes the tray entry.
type Config struct {
	URL    func() string // manager address, opened by click / "Open" (asked when used: the port can change)
	MapURL func() string // map page (the in-sim viewer in a browser tab)
	Status func() string // short line shown in the menu (e.g. simulator state)
	Quit   func()        // called when the user picks "Quit"; the tray exits afterwards
}

// Open opens a URL in the default browser.
func Open(url string) { _ = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start() }

// Run blocks until the user quits (or Stop is called). It must be called from the main goroutine.
func Run(c Config) {
	systray.Run(func() {
		systray.SetIcon(icon())
		systray.SetTitle("Open Charts")
		systray.SetTooltip("Open Charts")
		systray.SetOnTapped(func() { Open(c.URL()) })

		open := systray.AddMenuItem("Open manager / Abrir gerenciador", "")
		openMap := systray.AddMenuItem("Open map / Abrir mapa", "")
		status := systray.AddMenuItem("...", "")
		status.Disable()
		systray.AddSeparator()
		quit := systray.AddMenuItem("Quit / Sair", "")

		go func() {
			t := time.NewTicker(2 * time.Second)
			defer t.Stop()
			for {
				if c.Status != nil {
					s := c.Status()
					status.SetTitle(s)
					systray.SetTooltip("Open Charts - " + s)
				}
				select {
				case <-t.C:
				case <-open.ClickedCh:
					Open(c.URL())
				case <-openMap.ClickedCh:
					Open(c.MapURL())
				case <-quit.ClickedCh:
					if c.Quit != nil {
						c.Quit()
					}
					systray.Quit()
					return
				}
			}
		}()
	}, nil)
}

// Stop removes the icon and makes Run return.
func Stop() { systray.Quit() }
