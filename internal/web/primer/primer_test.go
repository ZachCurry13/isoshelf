package primer

import (
	"strings"
	"testing"
)

const sample = `
/** a comment */
[data-color-mode="light"][data-light-theme="light"],
[data-color-mode="auto"][data-light-theme="light"] {
  --fgColor-default: #1f2328; /** body text */
  --fgColor-muted: #59636e;
  --unused: #000000;
  --button-primary-borderColor-rest: var(--borderColor-translucent);
  --borderColor-translucent: #1f232826;
}
@media (prefers-color-scheme: dark) {
  [data-color-mode][data-color-mode="auto"][data-dark-theme="light"] {
    --fgColor-default: #1f2328;
    --unused: #000000;
  }
}
`

func TestParseFilterKeepsOnlyWhatIsWanted(t *testing.T) {
	root, err := Parse(sample)
	if err != nil {
		t.Fatal(err)
	}
	got := Filter(root, []string{"--fgColor-default"})
	if strings.Contains(got, "--unused") || strings.Contains(got, "--fgColor-muted") {
		t.Errorf("kept a name nobody asked for:\n%s", got)
	}
	// A selector list that ran over two lines is one rule, and a nested @media
	// survives with the rule inside it.
	for _, want := range []string{
		`[data-color-mode="light"][data-light-theme="light"], [data-color-mode="auto"][data-light-theme="light"] {`,
		"@media (prefers-color-scheme: dark) {",
		`[data-dark-theme="light"] {`,
		"--fgColor-default: #1f2328;",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
}

func TestAnEmptyBlockIsLeftOut(t *testing.T) {
	root, _ := Parse(sample)
	got := Filter(root, []string{"--fgColor-muted"})
	if strings.Contains(got, "@media") {
		t.Errorf("a block with nothing wanted in it was kept:\n%s", got)
	}
}

// A token wanted brings along what it is defined in terms of - even when a
// different theme gives the same name a plain value.
func TestCloseFollowsReferencesFromAnyTheme(t *testing.T) {
	a, _ := Parse(sample)
	b, _ := Parse("[data-x] {\n  --button-primary-borderColor-rest: #123456;\n}\n")
	got := Close([]string{"--button-primary-borderColor-rest"}, a, b) // b comes last, with no reference in it
	if strings.Join(got, ",") != "--borderColor-translucent,--button-primary-borderColor-rest" {
		t.Errorf("closure = %v", got)
	}
}

func TestWantedIsWhatIsUsedAndNotDefined(t *testing.T) {
	css := ":root { --text: var(--fgColor-default); --muted: var(--fgColor-muted); --fg: red; }\n.a { color: var(--text); background: var(--fg); }"
	got := Wanted(css)
	if strings.Join(got, ",") != "--fgColor-default,--fgColor-muted" {
		t.Errorf("wanted = %v", got)
	}
}

func TestDefinedFindsSeveralToALine(t *testing.T) {
	d := Defined(".s-ok { --fg: var(--ok-fg); --bgc: var(--ok-bg); }\n/* --nope: 1; */")
	if !d["--fg"] || !d["--bgc"] || d["--nope"] || d["--ok-fg"] {
		t.Errorf("defined = %v", d)
	}
}

func TestParseRefusesWhatItDoesNotUnderstand(t *testing.T) {
	if _, err := Parse("a {\n  color: red;\n}\n"); err == nil {
		t.Error("a rule that is not a custom property was accepted")
	}
	if _, err := Parse("a {\n  --x: 1;\n"); err == nil {
		t.Error("an unclosed block was accepted")
	}
}
