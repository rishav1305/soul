package browser

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
)

// ---------------------------------------------------------------------------
// Remote browser configuration
// ---------------------------------------------------------------------------

// RemoteConfig holds settings for connecting to a remote launcher.Manager
// instance (typically running on titan-pc). When Enabled, LaunchHeadless
// will attempt to start the browser on the remote machine before falling
// back to a local launch.
type RemoteConfig struct {
	Enabled     bool   // Whether remote launching is active.
	ManagerURL  string // WebSocket URL of the launcher.Manager (e.g. "ws://192.168.0.196:7317").
	ProfileBase string // Base directory for Chrome profiles on the REMOTE machine.
}

// remoteConfig is the package-level remote configuration. Nil means remote
// launching has not been configured and all launches are local.
var remoteConfig *RemoteConfig

// SetRemoteConfig stores cfg as the active remote browser configuration.
// Passing a config with Enabled=false has the same effect as never calling
// this function — all launches will be local.
func SetRemoteConfig(cfg RemoteConfig) {
	remoteConfig = &cfg
}

// ---------------------------------------------------------------------------
// Chrome binary lookup
// ---------------------------------------------------------------------------

// FindChromeBin returns the path to a Chrome/Chromium binary.
// Checks common locations including snap internals on Linux.
func FindChromeBin() string {
	// Try rod's built-in lookup first.
	if path, found := launcher.LookPath(); found {
		return path
	}
	// Check common binary names in PATH.
	for _, name := range []string{
		"google-chrome", "google-chrome-stable", "chromium-browser", "chromium",
	} {
		if p, err := exec.LookPath(name); err == nil {
			return p
		}
	}
	// Check snap chromium internal binary (Ubuntu).
	snap := "/snap/chromium/current/usr/lib/chromium-browser/chrome"
	if _, err := os.Stat(snap); err == nil {
		return snap
	}
	return ""
}

// ---------------------------------------------------------------------------
// Native (non-automated) launch — unchanged
// ---------------------------------------------------------------------------

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

	bin := FindChromeBin()
	if bin == "" {
		return nil, fmt.Errorf("no Chrome or Chromium binary found")
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

// ---------------------------------------------------------------------------
// Headless launch — remote-first with local fallback
// ---------------------------------------------------------------------------

// LaunchHeadless opens a headless Chrome instance using the persistent
// profile directory for the given platform. If a remote launcher.Manager
// is configured and reachable the browser is started on the remote machine;
// otherwise it falls back to a local launch.
func LaunchHeadless(platform string) (*rod.Browser, error) {
	if _, ok := PlatformURLs[platform]; !ok {
		return nil, fmt.Errorf("unknown platform: %s", platform)
	}

	// Try remote launch first.
	if remoteConfig != nil && remoteConfig.Enabled {
		remoteProfile := filepath.Join(remoteConfig.ProfileBase, platform)
		b, err := launchRemote(remoteProfile)
		if err == nil {
			return b, nil
		}
		log.Printf("remote launch failed, falling back to local: %v", err)
	}

	// Local fallback.
	localProfile, err := ProfileDir(platform)
	if err != nil {
		return nil, fmt.Errorf("profile dir: %w", err)
	}
	return launchLocal(localProfile)
}

// LaunchHeadlessNoPlatform opens a headless Chrome instance without a
// persistent profile directory. This is intended for one-shot tasks such
// as PDF generation where cookies and login state are not required.
func LaunchHeadlessNoPlatform() (*rod.Browser, error) {
	// Try remote launch first.
	if remoteConfig != nil && remoteConfig.Enabled {
		b, err := launchRemoteNoProfile()
		if err == nil {
			return b, nil
		}
		log.Printf("remote launch (no-profile) failed, falling back to local: %v", err)
	}

	// Local fallback.
	return launchLocalNoProfile()
}

// ---------------------------------------------------------------------------
// Remote launch helpers
// ---------------------------------------------------------------------------

// launchRemote connects to the configured launcher.Manager and starts a
// headless Chrome instance using profileDir as the user-data directory on
// the REMOTE machine.
func launchRemote(profileDir string) (*rod.Browser, error) {
	l, err := launcher.NewManaged(remoteConfig.ManagerURL)
	if err != nil {
		return nil, fmt.Errorf("new managed launcher: %w", err)
	}
	l = l.UserDataDir(profileDir).KeepUserDataDir()

	client, err := l.Client()
	if err != nil {
		return nil, fmt.Errorf("managed client: %w", err)
	}

	browser := rod.New().Client(client)
	if err := browser.Connect(); err != nil {
		return nil, fmt.Errorf("connect to remote chrome: %w", err)
	}
	return browser, nil
}

// launchRemoteNoProfile connects to the configured launcher.Manager and
// starts a headless Chrome instance with a temporary (ephemeral) profile.
func launchRemoteNoProfile() (*rod.Browser, error) {
	l, err := launcher.NewManaged(remoteConfig.ManagerURL)
	if err != nil {
		return nil, fmt.Errorf("new managed launcher: %w", err)
	}
	// No UserDataDir — the manager will create a temp directory.

	client, err := l.Client()
	if err != nil {
		return nil, fmt.Errorf("managed client: %w", err)
	}

	browser := rod.New().Client(client)
	if err := browser.Connect(); err != nil {
		return nil, fmt.Errorf("connect to remote chrome: %w", err)
	}
	return browser, nil
}

// ---------------------------------------------------------------------------
// Local launch helpers
// ---------------------------------------------------------------------------

// launchLocal starts a headless Chrome instance on the local machine using
// the given profileDir as the user-data directory.
func launchLocal(profileDir string) (*rod.Browser, error) {
	l := launcher.New().
		UserDataDir(profileDir).
		Headless(true).
		NoSandbox(true)
	if bin := FindChromeBin(); bin != "" {
		l = l.Bin(bin)
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

// launchLocalNoProfile starts a headless Chrome instance on the local
// machine with a temporary profile directory (no persistent state).
func launchLocalNoProfile() (*rod.Browser, error) {
	l := launcher.New().
		Headless(true).
		NoSandbox(true)
	if bin := FindChromeBin(); bin != "" {
		l = l.Bin(bin)
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

// ---------------------------------------------------------------------------
// Navigation utility — unchanged
// ---------------------------------------------------------------------------

// NavigateWithDelay navigates the page to the given URL and waits for the
// page to stabilise. It uses a page-level timeout so that SPAs that never
// fully stabilise (e.g. LinkedIn) don't block indefinitely.
func NavigateWithDelay(page *rod.Page, url string) error {
	page = page.Timeout(30 * time.Second)
	if err := page.Navigate(url); err != nil {
		return fmt.Errorf("navigate: %w", err)
	}
	// WaitStable waits for 2s of DOM inactivity. If the page never
	// settles (common with SPAs), the 30s timeout above will fire and
	// we treat the page as "good enough".
	if err := page.WaitStable(2 * time.Second); err != nil {
		log.Printf("WaitStable timed out for %s (proceeding anyway)", url)
	}
	return nil
}
