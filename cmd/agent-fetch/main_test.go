package main

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/firede/agent-fetch/internal/fetcher"
	"github.com/urfave/cli/v3"
)

func TestRouteToDefaultWeb(t *testing.T) {
	root := newRootCommand(fetcher.DefaultConfig())

	tests := []struct {
		name string
		in   []string
		want []string
	}{
		{
			name: "no args",
			in:   []string{"agent-fetch"},
			want: []string{"agent-fetch"},
		},
		{
			name: "url shorthand",
			in:   []string{"agent-fetch", "https://example.com"},
			want: []string{"agent-fetch", "web", "https://example.com"},
		},
		{
			name: "flag shorthand",
			in:   []string{"agent-fetch", "--mode", "static", "https://example.com"},
			want: []string{"agent-fetch", "web", "--mode", "static", "https://example.com"},
		},
		{
			name: "doctor subcommand untouched",
			in:   []string{"agent-fetch", "doctor", "--help"},
			want: []string{"agent-fetch", "doctor", "--help"},
		},
		{
			name: "web subcommand untouched",
			in:   []string{"agent-fetch", "web", "--mode", "raw", "https://example.com"},
			want: []string{"agent-fetch", "web", "--mode", "raw", "https://example.com"},
		},
		{
			name: "root help untouched",
			in:   []string{"agent-fetch", "--help"},
			want: []string{"agent-fetch", "--help"},
		},
		{
			name: "root version untouched",
			in:   []string{"agent-fetch", "--version"},
			want: []string{"agent-fetch", "--version"},
		},
		{
			name: "unknown token treated as web shorthand",
			in:   []string{"agent-fetch", "https://example.net"},
			want: []string{"agent-fetch", "web", "https://example.net"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := routeToDefaultWeb(tc.in, root)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("unexpected args rewrite\ngot:  %#v\nwant: %#v", got, tc.want)
			}
		})
	}
}

func TestRouteToDefaultWeb_RegisteredSubcommandUntouched(t *testing.T) {
	root := newRootCommand(fetcher.DefaultConfig())
	const testSubcommand = "__test_subcommand__"
	root.Commands = append(root.Commands, &cli.Command{Name: testSubcommand})

	in := []string{"agent-fetch", testSubcommand, "arg1"}
	want := []string{"agent-fetch", testSubcommand, "arg1"}
	got := routeToDefaultWeb(in, root)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected args rewrite\ngot:  %#v\nwant: %#v", got, want)
	}
}

func TestHelpOutputContracts(t *testing.T) {
	t.Run("root help shows default web options", func(t *testing.T) {
		var out strings.Builder
		err := runForTest([]string{"agent-fetch", "-h"}, &out, &out)
		if err != nil {
			t.Fatalf("run help failed: %v", err)
		}
		helpText := out.String()
		if !strings.Contains(helpText, "DEFAULT WEB OPTIONS:") {
			t.Fatalf("expected default web options section, got:\n%s", helpText)
		}
		if !strings.Contains(helpText, "--format string") {
			t.Fatalf("expected web options in root help, got:\n%s", helpText)
		}
	})

	t.Run("doctor help excludes web flags", func(t *testing.T) {
		var out strings.Builder
		err := runForTest([]string{"agent-fetch", "doctor", "-h"}, &out, &out)
		if err != nil {
			t.Fatalf("run doctor help failed: %v", err)
		}
		helpText := out.String()
		if !strings.Contains(helpText, "--browser-path string") {
			t.Fatalf("expected doctor option in doctor help, got:\n%s", helpText)
		}
		if strings.Contains(helpText, "--format string") {
			t.Fatalf("did not expect web flag in doctor help, got:\n%s", helpText)
		}
	})

	t.Run("web help shows fetch flags", func(t *testing.T) {
		cmd := newRootCommand(fetcher.DefaultConfig())
		var out strings.Builder
		cmd.Writer = &out
		cmd.ErrWriter = &out
		err := cmd.Run(context.Background(), routeToDefaultWeb([]string{"agent-fetch", "web", "-h"}, cmd))
		if err != nil {
			t.Fatalf("run web help failed: %v", err)
		}
		helpText := out.String()
		if !strings.Contains(helpText, "--format string") {
			t.Fatalf("expected fetch flag in web help, got:\n%s", helpText)
		}
	})
}
