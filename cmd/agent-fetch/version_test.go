package main

import (
	"runtime/debug"
	"testing"
)

func TestResolveVersionInfo_UsesLdflagsValuesFirst(t *testing.T) {
	info := &debug.BuildInfo{
		Main: debug.Module{
			Version: "v0.2.1",
		},
		Settings: []debug.BuildSetting{
			{Key: "vcs.revision", Value: "abc123"},
			{Key: "vcs.time", Value: "2026-02-19T00:00:00Z"},
		},
	}

	gotV, gotC, gotD := resolveVersionInfo("v0.2.1", "deadbeef", "2026-02-19T00:00:00Z", info)
	if gotV != "v0.2.1" || gotC != "deadbeef" || gotD != "2026-02-19T00:00:00Z" {
		t.Fatalf("unexpected version info: %q %q %q", gotV, gotC, gotD)
	}
}

func TestResolveVersionInfo_UsesBuildInfoVersionWhenDefaultIsDev(t *testing.T) {
	info := &debug.BuildInfo{
		Main: debug.Module{
			Version: "v0.2.1",
		},
	}

	gotV, gotC, gotD := resolveVersionInfo("dev", "none", "unknown", info)
	if gotV != "v0.2.1" {
		t.Fatalf("expected version v0.2.1, got %q", gotV)
	}
	if gotC != "none" || gotD != "unknown" {
		t.Fatalf("unexpected fallback values: %q %q", gotC, gotD)
	}
}

func TestResolveVersionInfo_UsesVCSSettingsForFallback(t *testing.T) {
	info := &debug.BuildInfo{
		Main: debug.Module{
			Version: "(devel)",
		},
		Settings: []debug.BuildSetting{
			{Key: "vcs.revision", Value: "abc123"},
			{Key: "vcs.time", Value: "2026-02-19T00:00:00Z"},
		},
	}

	gotV, gotC, gotD := resolveVersionInfo("dev", "none", "unknown", info)
	if gotV != "dev" || gotC != "abc123" || gotD != "2026-02-19T00:00:00Z" {
		t.Fatalf("unexpected version info: %q %q %q", gotV, gotC, gotD)
	}
}
