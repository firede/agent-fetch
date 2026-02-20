package main

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestDiagnoseBrowser_ResolveFails(t *testing.T) {
	deps := doctorDeps{
		resolveBrowser: func(string) (string, []string, error) {
			return "", nil, errors.New("not found")
		},
		runProbe: func(context.Context, string) (string, error) {
			t.Fatal("runProbe should not be called when resolve fails")
			return "", nil
		},
		goos:   "linux",
		goarch: "amd64",
	}

	check := diagnoseBrowser(context.Background(), deps, "")
	if check.status != doctorStatusWarn {
		t.Fatalf("expected warn status, got %q", check.status)
	}
	if check.err == nil || !strings.Contains(check.err.Error(), "not found") {
		t.Fatalf("unexpected error: %v", check.err)
	}
	if len(check.guidance) == 0 {
		t.Fatal("expected remediation guidance")
	}
}

func TestDiagnoseBrowser_ProbeSucceeds(t *testing.T) {
	deps := doctorDeps{
		resolveBrowser: func(string) (string, []string, error) {
			return "/usr/bin/google-chrome", []string{"/usr/bin/google-chrome"}, nil
		},
		runProbe: func(_ context.Context, bin string) (string, error) {
			if bin != "/usr/bin/google-chrome" {
				t.Fatalf("unexpected binary: %s", bin)
			}
			return "about:blank", nil
		},
		goos:   "linux",
		goarch: "amd64",
	}

	check := diagnoseBrowser(context.Background(), deps, "")
	if check.status != doctorStatusOK {
		t.Fatalf("expected ok status, got %q (err=%v)", check.status, check.err)
	}
	if check.selected != "/usr/bin/google-chrome" {
		t.Fatalf("unexpected selected binary: %q", check.selected)
	}
}

func TestDiagnoseBrowser_ProbeFails(t *testing.T) {
	deps := doctorDeps{
		resolveBrowser: func(string) (string, []string, error) {
			return "/usr/bin/google-chrome", []string{"/usr/bin/google-chrome"}, nil
		},
		runProbe: func(context.Context, string) (string, error) {
			return "missing libnss3.so", errors.New("exit status 1")
		},
		goos:   "linux",
		goarch: "amd64",
	}

	check := diagnoseBrowser(context.Background(), deps, "")
	if check.status != doctorStatusWarn {
		t.Fatalf("expected warn status, got %q", check.status)
	}
	if check.err == nil {
		t.Fatal("expected probe error")
	}
	if check.probeOutput == "" {
		t.Fatal("expected probe output tail")
	}
}

func TestBrowserGuidance_Override(t *testing.T) {
	g := browserGuidance("linux", "/custom/chrome")
	if len(g) == 0 {
		t.Fatal("expected override guidance")
	}
	if !strings.Contains(strings.Join(g, "\n"), "/custom/chrome") {
		t.Fatalf("expected override path in guidance, got %v", g)
	}
}
