package fetcher

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

var ErrBrowserExecutableNotFound = errors.New("no Chrome/Chromium executable found in known locations")

func ResolveBrowserExecutablePath(browserPath string) (string, []string, error) {
	return resolveBrowserExecutablePath(exec.LookPath, runtime.GOOS, os.Getenv("USERPROFILE"), browserPath)
}

func resolveBrowserExecutablePath(lookPath func(string) (string, error), goos, userProfile, browserPath string) (string, []string, error) {
	if lookPath == nil {
		return "", nil, errors.New("lookPath is nil")
	}

	override := strings.TrimSpace(browserPath)
	if override != "" {
		path, err := lookPath(override)
		if err != nil || path == "" {
			return "", nil, fmt.Errorf("browser path %q is not executable or not found: %w", override, err)
		}
		return path, []string{path}, nil
	}

	candidates := browserExecutableCandidates(goos, userProfile)
	found := make([]string, 0, len(candidates))
	seen := make(map[string]struct{}, len(candidates))
	for _, candidate := range candidates {
		path, err := lookPath(candidate)
		if err != nil || path == "" {
			continue
		}
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		found = append(found, path)
	}
	if len(found) == 0 {
		return "", nil, ErrBrowserExecutableNotFound
	}
	return found[0], found, nil
}

func browserExecutableCandidates(goos, userProfile string) []string {
	switch goos {
	case "darwin":
		return []string{
			"/Applications/Chromium.app/Contents/MacOS/Chromium",
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			// Homebrew chromium symlinks (Apple Silicon / Intel).
			"/opt/homebrew/bin/chromium",
			"/usr/local/bin/chromium",
		}
	case "windows":
		candidates := []string{
			"chrome",
			"chrome.exe",
			`C:\Program Files (x86)\Google\Chrome\Application\chrome.exe`,
			`C:\Program Files\Google\Chrome\Application\chrome.exe`,
		}
		if userProfile != "" {
			candidates = append(candidates,
				filepath.Join(userProfile, `AppData\Local\Google\Chrome\Application\chrome.exe`),
				filepath.Join(userProfile, `AppData\Local\Chromium\Application\chrome.exe`),
			)
		}
		return candidates
	default:
		return []string{
			"headless_shell",
			"headless-shell",
			"chromium",
			"chromium-browser",
			"google-chrome",
			"google-chrome-stable",
			"google-chrome-beta",
			"google-chrome-unstable",
			"/usr/bin/google-chrome",
			"/usr/local/bin/chrome",
			"/snap/bin/chromium",
			"chrome",
		}
	}
}
