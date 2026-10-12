// mkicon writes the application icon (the same one shown in the tray) to a file.
package main

import (
	"log"
	"os"

	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/tray"
)

func main() {
	if len(os.Args) != 2 {
		log.Fatal("usage: mkicon FILE.ico")
	}
	if err := os.WriteFile(os.Args[1], tray.ICO(), 0o644); err != nil {
		log.Fatal(err)
	}
}
