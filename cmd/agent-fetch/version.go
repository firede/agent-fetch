package main

import (
	"fmt"
	"runtime/debug"
	"strings"
)

func versionString() string {
	v, c, d := resolvedVersionInfo()
	return fmt.Sprintf("%s (%s, %s)", v, c, d)
}

func resolvedVersionInfo() (string, string, string) {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return normalizeVersionParts(version, commit, date)
	}
	return resolveVersionInfo(version, commit, date, info)
}

func resolveVersionInfo(v, c, d string, info *debug.BuildInfo) (string, string, string) {
	v, c, d = normalizeVersionParts(v, c, d)
	if info == nil {
		return v, c, d
	}

	if (v == "dev" || v == "") && info.Main.Version != "" && info.Main.Version != "(devel)" {
		v = info.Main.Version
	}
	if c == "none" || c == "" {
		if rev := buildSetting(info, "vcs.revision"); rev != "" {
			c = rev
		}
	}
	if d == "unknown" || d == "" {
		if t := buildSetting(info, "vcs.time"); t != "" {
			d = t
		}
	}

	return normalizeVersionParts(v, c, d)
}

func buildSetting(info *debug.BuildInfo, key string) string {
	for _, item := range info.Settings {
		if item.Key == key {
			return item.Value
		}
	}
	return ""
}

func normalizeVersionParts(v, c, d string) (string, string, string) {
	v = strings.TrimSpace(v)
	c = strings.TrimSpace(c)
	d = strings.TrimSpace(d)

	if v == "" {
		v = "dev"
	}
	if c == "" {
		c = "none"
	}
	if d == "" {
		d = "unknown"
	}
	return v, c, d
}
