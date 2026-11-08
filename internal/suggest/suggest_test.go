package suggest

import (
	"reflect"
	"testing"
)

func TestClosest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		input      string
		candidates []string
		want       string
		wantOK     bool
	}{
		{
			name:       "databse to database",
			input:      "databse",
			candidates: []string{"database", "server", "host"},
			want:       "database",
			wantOK:     true,
		},
		{
			name:       "paralel to parallel",
			input:      "paralel",
			candidates: []string{"parallel", "sequential"},
			want:       "parallel",
			wantOK:     true,
		},
		{
			name:       "hsot to host transposition",
			input:      "hsot",
			candidates: []string{"host", "port"},
			want:       "host",
			wantOK:     true,
		},
		{
			name:       "prot to port transposition",
			input:      "prot",
			candidates: []string{"port", "protocol"},
			want:       "port",
			wantOK:     true,
		},
		{
			name:       "Database to database case-only",
			input:      "Database",
			candidates: []string{"database", "server"},
			want:       "database",
			wantOK:     true,
		},
		{
			name:       "ZP_DATABSE_HOST to ZP_DATABASE_HOST",
			input:      "ZP_DATABSE_HOST",
			candidates: []string{"ZP_DATABASE_HOST", "ZP_DATABASE_PORT"},
			want:       "ZP_DATABASE_HOST",
			wantOK:     true,
		},
		{
			name:       "exact match skipped",
			input:      "host",
			candidates: []string{"host", "hosts"},
			want:       "hosts",
			wantOK:     true,
		},
		{
			name:       "exact match only returns false",
			input:      "host",
			candidates: []string{"host"},
			want:       "",
			wantOK:     false,
		},
		{
			name:       "no suggestion for xyz among database and server",
			input:      "xyz",
			candidates: []string{"database", "server"},
			want:       "",
			wantOK:     false,
		},
		{
			name:       "tie broken by order",
			input:      "post",
			candidates: []string{"cost", "host", "port", "past"},
			want:       "cost",
			wantOK:     true,
		},
		{
			name:       "empty name",
			input:      "",
			candidates: []string{"database", "server"},
			want:       "",
			wantOK:     false,
		},
		{
			name:       "empty candidates",
			input:      "host",
			candidates: []string{},
			want:       "",
			wantOK:     false,
		},
		{
			name:       "nil candidates",
			input:      "host",
			candidates: nil,
			want:       "",
			wantOK:     false,
		},
		{
			name:       "empty name and nil candidates",
			input:      "",
			candidates: nil,
			want:       "",
			wantOK:     false,
		},
		{
			name:       "empty string candidate skipped",
			input:      "a",
			candidates: []string{""},
			want:       "",
			wantOK:     false,
		},
		{
			name:       "multi-byte runes distance 1",
			input:      "héllo",
			candidates: []string{"hello", "world"},
			want:       "hello",
			wantOK:     true,
		},
		{
			name:       "multi-byte runes reverse",
			input:      "hello",
			candidates: []string{"héllo", "world"},
			want:       "héllo",
			wantOK:     true,
		},
		{
			name:       "case-insensitive match prioritized over distance",
			input:      "port",
			candidates: []string{"post", "PORT"},
			want:       "PORT",
			wantOK:     true,
		},
		{
			name:       "case-insensitive match first in order",
			input:      "port",
			candidates: []string{"PORT", "Port"},
			want:       "PORT",
			wantOK:     true,
		},
		{
			name:       "distance limit n=5 limit is 1 accepted",
			input:      "abcde",
			candidates: []string{"abcdf"},
			want:       "abcdf",
			wantOK:     true,
		},
		{
			name:       "distance limit n=5 limit is 1 rejected",
			input:      "abcde",
			candidates: []string{"abcfg"},
			want:       "",
			wantOK:     false,
		},
		{
			name:       "distance limit n=6 limit is 2 accepted",
			input:      "abcdef",
			candidates: []string{"abcdgh"},
			want:       "abcdgh",
			wantOK:     true,
		},
		{
			name:       "distance limit n=6 limit is 2 rejected",
			input:      "abcdef",
			candidates: []string{"abcghi"},
			want:       "",
			wantOK:     false,
		},
		{
			name:       "distance limit n=10 limit is 3 accepted",
			input:      "abcdefghij",
			candidates: []string{"abcdefgxyz"},
			want:       "abcdefgxyz",
			wantOK:     true,
		},
		{
			name:       "distance limit n=10 limit is 3 rejected",
			input:      "abcdefghij",
			candidates: []string{"abcdefwxyz"},
			want:       "",
			wantOK:     false,
		},
		{
			name:       "distance limit cap at 4 accepted",
			input:      "abcdefghijklmnopqrstuvwxy",
			candidates: []string{"abcdefghijklmnopqrstu1234"},
			want:       "abcdefghijklmnopqrstu1234",
			wantOK:     true,
		},
		{
			name:       "distance limit cap at 4 rejected",
			input:      "abcdefghijklmnopqrstuvwxy",
			candidates: []string{"abcdefghijklmnopqrst12345"},
			want:       "",
			wantOK:     false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, ok := Closest(tt.input, tt.candidates)
			if ok != tt.wantOK {
				t.Fatalf("Closest(%q, %v) ok = %v, want %v", tt.input, tt.candidates, ok, tt.wantOK)
			}
			if got != tt.want {
				t.Fatalf("Closest(%q, %v) = %q, want %q", tt.input, tt.candidates, got, tt.want)
			}
		})
	}
}

func TestClosestCandidatesUnchanged(t *testing.T) {
	t.Parallel()

	orig := []string{"database", "host", "port", "server"}
	candidates := make([]string, len(orig))
	copy(candidates, orig)

	got, ok := Closest("hsot", candidates)
	if !ok || got != "host" {
		t.Fatalf("Closest(\"hsot\", %v) = (%q, %v), want (\"host\", true)", candidates, got, ok)
	}

	if !reflect.DeepEqual(candidates, orig) {
		t.Fatalf("candidates slice was modified: got %v, want %v", candidates, orig)
	}
}

func TestDistance(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		a    string
		b    string
		want int
	}{
		{
			name: "both empty",
			a:    "",
			b:    "",
			want: 0,
		},
		{
			name: "a empty b abc",
			a:    "",
			b:    "abc",
			want: 3,
		},
		{
			name: "a abc b empty",
			a:    "abc",
			b:    "",
			want: 3,
		},
		{
			name: "identical",
			a:    "database",
			b:    "database",
			want: 0,
		},
		{
			name: "single substitution",
			a:    "cat",
			b:    "bat",
			want: 1,
		},
		{
			name: "single insertion",
			a:    "databse",
			b:    "database",
			want: 1,
		},
		{
			name: "single deletion",
			a:    "parallel",
			b:    "paralel",
			want: 1,
		},
		{
			name: "adjacent transposition hsot to host",
			a:    "hsot",
			b:    "host",
			want: 1,
		},
		{
			name: "adjacent transposition prot to port",
			a:    "prot",
			b:    "port",
			want: 1,
		},
		{
			name: "OSA ca to abc is 3 not 2",
			a:    "ca",
			b:    "abc",
			want: 3,
		},
		{
			name: "OSA abc to ca is 3 not 2",
			a:    "abc",
			b:    "ca",
			want: 3,
		},
		{
			name: "multi-byte rune substitution héllo to hello",
			a:    "héllo",
			b:    "hello",
			want: 1,
		},
		{
			name: "multi-byte rune café to cafe",
			a:    "café",
			b:    "cafe",
			want: 1,
		},
		{
			name: "multi-byte rune transposition",
			a:    "日本語",
			b:    "日語本",
			want: 1,
		},
		{
			name: "multi-byte rune deletion",
			a:    "日本語",
			b:    "日本",
			want: 1,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := Distance(tt.a, tt.b)
			if got != tt.want {
				t.Fatalf("Distance(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestDistanceSymmetric(t *testing.T) {
	t.Parallel()

	pairs := [][2]string{
		{"", ""},
		{"", "abc"},
		{"ca", "abc"},
		{"hsot", "host"},
		{"prot", "port"},
		{"héllo", "hello"},
		{"café", "cafe"},
		{"ZP_DATABSE_HOST", "ZP_DATABASE_HOST"},
		{"database", "server"},
		{"kitten", "sitting"},
		{"rosettacode", "raisethysword"},
		{"日本語", "日語本"},
	}

	for _, pair := range pairs {
		pair := pair
		t.Run(pair[0]+"_vs_"+pair[1], func(t *testing.T) {
			t.Parallel()
			d1 := Distance(pair[0], pair[1])
			d2 := Distance(pair[1], pair[0])
			if d1 != d2 {
				t.Fatalf("Distance is not symmetric for %q and %q: %d != %d", pair[0], pair[1], d1, d2)
			}
		})
	}
}
