package main

import (
	"context"
	"fmt"
	"io"
	"runtime"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/firede/agent-fetch/internal/fetcher"
)

const (
	doctorProbeTimeout = 8 * time.Second
	maxProbeOutputTail = 800
)

type doctorStatus string

const (
	doctorStatusOK   doctorStatus = "ok"
	doctorStatusWarn doctorStatus = "warn"
)

type doctorDeps struct {
	resolveBrowser func(string) (string, []string, error)
	runProbe       func(context.Context, string) (string, error)
	goos           string
	goarch         string
}

type browserCheck struct {
	status      doctorStatus
	candidates  []string
	selected    string
	probeOutput string
	err         error
	guidance    []string
}

func runDoctor(ctx context.Context, out io.Writer, browserPath string) (doctorStatus, error) {
	deps := doctorDeps{
		resolveBrowser: fetcher.ResolveBrowserExecutablePath,
		runProbe:       runBrowserProbe,
		goos:           runtime.GOOS,
		goarch:         runtime.GOARCH,
	}

	check := diagnoseBrowser(ctx, deps, browserPath)

	if _, err := fmt.Fprintf(out, "version: %s\n", versionString()); err != nil {
		return doctorStatusWarn, err
	}
	if _, err := fmt.Fprintf(out, "platform: %s/%s\n", deps.goos, deps.goarch); err != nil {
		return doctorStatusWarn, err
	}
	if strings.TrimSpace(browserPath) != "" {
		if _, err := fmt.Fprintf(out, "browser path override: %s\n", browserPath); err != nil {
			return doctorStatusWarn, err
		}
	}

	if check.status == doctorStatusOK {
		if _, err := fmt.Fprintln(out, "browser mode: ready"); err != nil {
			return doctorStatusWarn, err
		}
		if _, err := fmt.Fprintf(out, "browser binary: %s\n", check.selected); err != nil {
			return doctorStatusWarn, err
		}
	} else {
		if _, err := fmt.Fprintln(out, "browser mode: not ready"); err != nil {
			return doctorStatusWarn, err
		}
		if len(check.candidates) > 0 {
			if _, err := fmt.Fprintf(out, "browser candidates found: %s\n", strings.Join(check.candidates, ", ")); err != nil {
				return doctorStatusWarn, err
			}
		} else {
			if _, err := fmt.Fprintln(out, "browser candidates found: none"); err != nil {
				return doctorStatusWarn, err
			}
		}
		if check.err != nil {
			if _, err := fmt.Fprintf(out, "probe error: %v\n", check.err); err != nil {
				return doctorStatusWarn, err
			}
		}
		if check.probeOutput != "" {
			if _, err := fmt.Fprintf(out, "probe output (tail): %q\n", check.probeOutput); err != nil {
				return doctorStatusWarn, err
			}
		}
		if len(check.guidance) > 0 {
			if _, err := fmt.Fprintln(out, "recommended fixes:"); err != nil {
				return doctorStatusWarn, err
			}
			for i, line := range check.guidance {
				if _, err := fmt.Fprintf(out, "%d. %s\n", i+1, line); err != nil {
					return doctorStatusWarn, err
				}
			}
		}
	}
	return check.status, nil
}

func diagnoseBrowser(ctx context.Context, deps doctorDeps, browserPath string) browserCheck {
	selected, found, err := deps.resolveBrowser(browserPath)
	if err != nil {
		return browserCheck{
			status:     doctorStatusWarn,
			candidates: found,
			err:        err,
			guidance:   browserGuidance(deps.goos, browserPath),
		}
	}

	probeCtx, cancel := context.WithTimeout(ctx, doctorProbeTimeout)
	out, err := deps.runProbe(probeCtx, selected)
	cancel()
	if err == nil {
		return browserCheck{
			status:     doctorStatusOK,
			candidates: found,
			selected:   selected,
		}
	}

	lastOut := clampTail(out, maxProbeOutputTail)
	if lastOut == "" {
		lastOut = clampTail(err.Error(), maxProbeOutputTail)
	}
	return browserCheck{
		status:      doctorStatusWarn,
		candidates:  found,
		selected:    selected,
		probeOutput: lastOut,
		err:         err,
		guidance:    browserGuidance(deps.goos, browserPath),
	}
}

func runBrowserProbe(ctx context.Context, binary string) (string, error) {
	allocOpts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.ExecPath(binary),
		chromedp.NoDefaultBrowserCheck,
		chromedp.NoFirstRun,
	)
	allocCtx, cancelAlloc := chromedp.NewExecAllocator(ctx, allocOpts...)
	defer cancelAlloc()

	tabCtx, cancelTab := chromedp.NewContext(allocCtx)
	defer cancelTab()

	var finalURL string
	if err := chromedp.Run(tabCtx,
		chromedp.Navigate("about:blank"),
		chromedp.Location(&finalURL),
	); err != nil {
		return "", err
	}
	return finalURL, nil
}

func clampTail(s string, max int) string {
	trimmed := strings.TrimSpace(strings.ReplaceAll(s, "\x00", ""))
	if max <= 0 || len(trimmed) <= max {
		return trimmed
	}
	return trimmed[len(trimmed)-max:]
}

func browserGuidance(goos, browserPath string) []string {
	override := strings.TrimSpace(browserPath)
	if override != "" {
		return []string{
			"Verify the override path is executable in the current environment: " + override,
			"In containers, ensure the browser binary and required shared libraries are installed in the same image layer.",
			"Run doctor with a known path if needed: agent-fetch --doctor --browser-path /path/to/chrome.",
		}
	}

	switch goos {
	case "linux":
		return []string{
			"Install Chrome/Chromium with your distro package manager and ensure the executable is discoverable.",
			"For containerized workloads, set --browser-path explicitly to the browser binary in the image (for example /usr/bin/chromium).",
			"If startup fails with missing shared libraries, install runtime deps such as libnss3, libatk-bridge2.0-0, libgtk-3-0, libgbm1, and fonts.",
		}
	case "darwin":
		return []string{
			"Install Google Chrome or Chromium in /Applications, or provide an explicit --browser-path override.",
			"If Chrome is installed in a custom location, run with --browser-path '<full path to Chrome binary>'.",
		}
	case "windows":
		return []string{
			"Install Google Chrome or Chromium and ensure the executable is discoverable, or pass --browser-path.",
			"Verify the configured path from the same terminal session used to run agent-fetch.",
		}
	}
	return []string{
		"Install Chrome/Chromium and ensure the executable is discoverable, or pass --browser-path.",
	}
}
