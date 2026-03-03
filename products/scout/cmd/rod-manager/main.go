package main

import (
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"strings"

	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/launcher/flags"
)

const allowedPrefix = "/.soul/scout/profiles/"

func findChromeBin() string {
	if p, ok := launcher.LookPath(); ok {
		return p
	}
	for _, name := range []string{"google-chrome", "google-chrome-stable", "chromium-browser", "chromium"} {
		if p, err := exec.LookPath(name); err == nil {
			return p
		}
	}
	return ""
}

func main() {
	m := launcher.NewManager()
	m.BeforeLaunch = func(l *launcher.Launcher, w http.ResponseWriter, r *http.Request) {
		if dir := l.Get(flags.UserDataDir); dir != "" && !strings.Contains(dir, allowedPrefix) {
			http.Error(w, "user-data-dir not under "+allowedPrefix, http.StatusForbidden)
			panic(http.ErrAbortHandler)
		}
		l.Headless(true).NoSandbox(true)
		if bin := findChromeBin(); bin != "" {
			l.Bin(bin)
		}
	}
	addr := "127.0.0.1:7317"
	fmt.Println("rod-manager listening on", addr)
	log.Fatal(http.ListenAndServe(addr, m))
}
