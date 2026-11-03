package catalog

import (
	"slices"
	"testing"
)

func TestPlaceholders(t *testing.T) {
	tests := []struct {
		tmpl string
		want []string
	}{
		{"https://example.org/{cycle}/{version}/", []string{"cycle", "version"}},
		{"{version}-{version}", []string{"version"}},
		{`MX-\d{2}\.\d{1,2}_x64\.iso`, nil}, // regex quantifiers
		{`\p{Greek}+-{version}`, []string{"version"}},
		{"no placeholders", nil},
	}
	for _, tt := range tests {
		if got := Placeholders(tt.tmpl); !slices.Equal(got, tt.want) {
			t.Errorf("Placeholders(%q) = %q, want %q", tt.tmpl, got, tt.want)
		}
	}
}

func TestExpand(t *testing.T) {
	got, err := Expand("https://example.org/{cycle}/{file}.sha256", map[string]string{
		"cycle": "24.04",
		"file":  "example-24.04.1.iso",
	})
	if err != nil {
		t.Fatal(err)
	}
	if want := "https://example.org/24.04/example-24.04.1.iso.sha256"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}

	if _, err := Expand("{version}", nil); err == nil {
		t.Error("Expand with a missing value: want an error")
	}
}

func TestExpandRegexp(t *testing.T) {
	re, err := ExpandRegexp(`linuxmint-{version}-cinnamon-64bit\.iso`, map[string]string{"version": "22.3"})
	if err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]bool{
		"linuxmint-22.3-cinnamon-64bit.iso":     true,
		"linuxmint-2203-cinnamon-64bit.iso":     false, // "." in the version is literal
		"old-linuxmint-22.3-cinnamon-64bit.iso": false, // whole name only
		"linuxmint-22.3-cinnamon-64bit.iso.zip": false,
	} {
		if got := re.MatchString(name); got != want {
			t.Errorf("match %q = %v, want %v", name, got, want)
		}
	}
}

func TestLiteralFileIsAPatternThatNamesOneFile(t *testing.T) {
	vars := map[string]string{"version": "2.0.5"}
	if got, ok := LiteralFile(`Example-{version}-amd64\.iso`, vars); !ok || got != "Example-2.0.5-amd64.iso" {
		t.Errorf("got %q, %v", got, ok)
	}
	// A group around a literal is still one file.
	if got, ok := LiteralFile(`Example-{version}-(?P<arch>amd64)\.iso`, vars); !ok || got != "Example-2.0.5-amd64.iso" {
		t.Errorf("a group around a literal: got %q, %v", got, ok)
	}
	for _, tmpl := range []string{`Example-{version}-(?:amd64|arm64)\.iso`, `Example-{version}-[a-z]+\.iso`, `Example-{nope}\.iso`} {
		if got, ok := LiteralFile(tmpl, vars); ok {
			t.Errorf("%s is not one file, but gave %q", tmpl, got)
		}
	}
}
