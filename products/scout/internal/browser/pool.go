package browser

import (
	"fmt"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
)

// LaunchVisible opens a visible (non-headless) Chrome instance using the
// persistent profile directory for the given platform, then navigates to
// the platform's login URL. The caller is responsible for closing the
// returned browser when done.
func LaunchVisible(platform string) (*rod.Browser, *rod.Page, error) {
	urls, ok := PlatformURLs[platform]
	if !ok {
		return nil, nil, fmt.Errorf("unknown platform: %s", platform)
	}

	profileDir, err := ProfileDir(platform)
	if err != nil {
		return nil, nil, fmt.Errorf("profile dir: %w", err)
	}

	l := launcher.New().
		UserDataDir(profileDir).
		Headless(false).
		NoSandbox(true)
	if path, found := launcher.LookPath(); found {
		l = l.Bin(path)
	}
	u, err := l.Launch()
	if err != nil {
		return nil, nil, fmt.Errorf("launch chrome: %w", err)
	}

	browser := rod.New().ControlURL(u)
	if err := browser.Connect(); err != nil {
		return nil, nil, fmt.Errorf("connect to chrome: %w", err)
	}

	page, err := browser.Page(proto.TargetCreateTarget{})
	if err != nil {
		browser.MustClose()
		return nil, nil, fmt.Errorf("new page: %w", err)
	}

	if err := NavigateWithDelay(page, urls.Login); err != nil {
		browser.MustClose()
		return nil, nil, fmt.Errorf("navigate to login: %w", err)
	}

	return browser, page, nil
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
