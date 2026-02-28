package browser

import (
	"fmt"
	"os/exec"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
)

// LaunchNative opens a regular (non-automated) Chrome process for manual
// login. This avoids automation flags that trigger bot detection on sites
// like LinkedIn. The caller receives the *exec.Cmd so it can wait on exit.
func LaunchNative(platform string) (*exec.Cmd, error) {
	urls, ok := PlatformURLs[platform]
	if !ok {
		return nil, fmt.Errorf("unknown platform: %s", platform)
	}

	profileDir, err := ProfileDir(platform)
	if err != nil {
		return nil, fmt.Errorf("profile dir: %w", err)
	}

	// Find Chrome/Chromium binary.
	bin := ""
	for _, candidate := range []string{
		"google-chrome", "google-chrome-stable", "chromium-browser", "chromium",
	} {
		if p, err := exec.LookPath(candidate); err == nil {
			bin = p
			break
		}
	}
	if bin == "" {
		return nil, fmt.Errorf("no Chrome or Chromium binary found in PATH")
	}

	cmd := exec.Command(bin,
		"--user-data-dir="+profileDir,
		"--no-first-run",
		"--no-default-browser-check",
		"--no-sandbox",
		urls.Login,
	)
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start chrome: %w", err)
	}

	return cmd, nil
}

// LaunchHeadless opens a headless Chrome instance using the persistent
// profile directory for the given platform. This is intended for
// automated scraping after the user has already logged in via
// LaunchVisible. The caller is responsible for closing the browser.
func LaunchHeadless(platform string) (*rod.Browser, error) {
	if _, ok := PlatformURLs[platform]; !ok {
		return nil, fmt.Errorf("unknown platform: %s", platform)
	}

	profileDir, err := ProfileDir(platform)
	if err != nil {
		return nil, fmt.Errorf("profile dir: %w", err)
	}

	l := launcher.New().
		UserDataDir(profileDir).
		Headless(true).
		NoSandbox(true)
	if path, found := launcher.LookPath(); found {
		l = l.Bin(path)
	}
	u, err := l.Launch()
	if err != nil {
		return nil, fmt.Errorf("launch chrome: %w", err)
	}

	browser := rod.New().ControlURL(u)
	if err := browser.Connect(); err != nil {
		return nil, fmt.Errorf("connect to chrome: %w", err)
	}

	return browser, nil
}

// NavigateWithDelay navigates the page to the given URL and waits for the
// page to stabilise. It uses WaitStable with a 2-second threshold so that
// dynamic SPAs have time to finish rendering.
func NavigateWithDelay(page *rod.Page, url string) error {
	if err := page.Navigate(url); err != nil {
		return fmt.Errorf("navigate: %w", err)
	}
	if err := page.WaitStable(2 * time.Second); err != nil {
		return fmt.Errorf("wait stable: %w", err)
	}
	return nil
}
