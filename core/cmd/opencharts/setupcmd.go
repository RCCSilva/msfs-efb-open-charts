package main

import (
	"fmt"
	"os"
	"strings"

	"localmovingmap/internal/setup"
)

// runSetupCommand handles "opencharts setup ...", used by the installer (and by hand):
//
//	setup install-efb [URL]   install/update the EFB app in every detected MSFS installation
//	setup autostart on|off    add/remove the "start with MSFS" entry
//	setup uninstall           remove the EFB app and the autostart entry (the library is kept)
//
// It returns the process exit code.
func runSetupCommand(args []string) int {
	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	installs := setup.Detect(exe)
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: opencharts setup install-efb [URL] | autostart on|off | uninstall")
		return 2
	}
	fail := 0
	switch args[0] {
	case "install-efb":
		url := "http://127.0.0.1:8080/"
		if len(args) > 1 {
			url = args[1]
		}
		for _, in := range installs {
			if err := setup.InstallEFB(in.Community, url); err != nil {
				fmt.Fprintln(os.Stderr, in.Community+":", err)
				fail = 1
			}
		}
	case "autostart":
		if len(args) < 2 || (args[1] != "on" && args[1] != "off") {
			fmt.Fprintln(os.Stderr, "usage: opencharts setup autostart on|off")
			return 2
		}
		for _, in := range installs {
			if err := setup.SetAutoStart(in.ExeXML, exe, nil, args[1] == "on"); err != nil {
				fmt.Fprintln(os.Stderr, in.ExeXML+":", err)
				fail = 1
			}
		}
	case "uninstall":
		for _, in := range installs {
			if err := setup.RemoveEFB(in.Community); err != nil {
				fmt.Fprintln(os.Stderr, in.Community+":", err)
				fail = 1
			}
			if in.AutoStart {
				if err := setup.SetAutoStart(in.ExeXML, exe, nil, false); err != nil {
					fmt.Fprintln(os.Stderr, in.ExeXML+":", err)
					fail = 1
				}
			}
		}
	default:
		fmt.Fprintln(os.Stderr, "unknown setup command: "+strings.Join(args, " "))
		return 2
	}
	return fail
}
