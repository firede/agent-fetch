package main

import (
	"reflect"
	"testing"
)

func TestRouteToDefaultWeb(t *testing.T) {
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
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := routeToDefaultWeb(tc.in)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("unexpected args rewrite\ngot:  %#v\nwant: %#v", got, tc.want)
			}
		})
	}
}
