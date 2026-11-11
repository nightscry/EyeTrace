package main

import (
	"os"

	"eyetracker/gui"
)

// usage: eyetracker [address]   (an address analyses it right after the splash)
func main() {
	addr := ""
	if len(os.Args) > 1 {
		addr = os.Args[1]
	}
	gui.RunGUI(addr)
}
