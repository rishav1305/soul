package browser

import (
	"os"
	"path/filepath"
)

// PlatformURLs maps each supported platform to its login and jobs URLs.
var PlatformURLs = map[string]struct {
	Login string
	Jobs  string
}{
	"linkedin": {
		Login: "https://www.linkedin.com/login",
		Jobs:  "https://www.linkedin.com/jobs/",
	},
	"naukri": {
		Login: "https://www.naukri.com/nlogin/login",
		Jobs:  "https://www.naukri.com/mnjuser/homepage",
	},
	"indeed": {
		Login: "https://secure.indeed.com/auth",
		Jobs:  "https://www.indeed.com/",
	},
	"wellfound": {
		Login: "https://wellfound.com/login",
		Jobs:  "https://wellfound.com/jobs",
	},
	"instahyre": {
		Login: "https://www.instahyre.com/login/",
		Jobs:  "https://www.instahyre.com/candidate/opportunities/",
	},
}

// AllPlatforms returns all supported platform names.
func AllPlatforms() []string {
	return []string{"linkedin", "naukri", "indeed", "wellfound", "instahyre"}
}

// ProfileDir returns the path to the Chrome user-data directory for the given
// platform, creating it if it does not exist. The directory lives under
// ~/.soul/scout/profiles/<platform>/.
func ProfileDir(platform string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".soul", "scout", "profiles", platform)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

// HasProfile reports whether the profile directory for the given platform
// already contains entries (i.e. a previous Chrome session wrote data there).
func HasProfile(platform string) bool {
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	dir := filepath.Join(home, ".soul", "scout", "profiles", platform)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	return len(entries) > 0
}
