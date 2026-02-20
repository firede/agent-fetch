package fetcher

import (
	"errors"
	"strings"
	"testing"
)

func TestResolveBrowserExecutablePath_Override(t *testing.T) {
	gotPath, gotFound, err := resolveBrowserExecutablePath(
		func(name string) (string, error) {
			if name == "/opt/chrome/chrome" {
				return "/opt/chrome/chrome", nil
			}
			return "", errors.New("not found")
		},
		"linux",
		"",
		"/opt/chrome/chrome",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotPath != "/opt/chrome/chrome" {
		t.Fatalf("unexpected path: %q", gotPath)
	}
	if len(gotFound) != 1 || gotFound[0] != "/opt/chrome/chrome" {
		t.Fatalf("unexpected found list: %v", gotFound)
	}
}

func TestResolveBrowserExecutablePath_OverrideMissing(t *testing.T) {
	_, _, err := resolveBrowserExecutablePath(
		func(string) (string, error) {
			return "", errors.New("not found")
		},
		"linux",
		"",
		"/opt/chrome/chrome",
	)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "browser path") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResolveBrowserExecutablePath_FindsDarwinBundle(t *testing.T) {
	const chromeBundle = "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
	gotPath, gotFound, err := resolveBrowserExecutablePath(
		func(name string) (string, error) {
			if name == chromeBundle {
				return chromeBundle, nil
			}
			return "", errors.New("not found")
		},
		"darwin",
		"",
		"",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotPath != chromeBundle {
		t.Fatalf("unexpected selected path: %q", gotPath)
	}
	if len(gotFound) != 1 || gotFound[0] != chromeBundle {
		t.Fatalf("unexpected found list: %v", gotFound)
	}
}

func TestResolveBrowserExecutablePath_NotFound(t *testing.T) {
	_, _, err := resolveBrowserExecutablePath(
		func(string) (string, error) {
			return "", errors.New("not found")
		},
		"linux",
		"",
		"",
	)
	if !errors.Is(err, ErrBrowserExecutableNotFound) {
		t.Fatalf("expected ErrBrowserExecutableNotFound, got %v", err)
	}
}

func TestBrowserExecutableCandidates_DarwinIncludesHomebrew(t *testing.T) {
	got := browserExecutableCandidates("darwin", "")
	joined := strings.Join(got, "\n")
	if !strings.Contains(joined, "/opt/homebrew/bin/chromium") {
		t.Fatalf("expected /opt/homebrew/bin/chromium in darwin candidates, got %v", got)
	}
	if !strings.Contains(joined, "/usr/local/bin/chromium") {
		t.Fatalf("expected /usr/local/bin/chromium in darwin candidates, got %v", got)
	}
}
