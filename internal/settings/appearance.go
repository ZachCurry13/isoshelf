package settings

// Appearance is how the page looks. Every field's zero value is the default,
// so a settings file that says nothing about appearance gets what suits most
// people: the light or dark the computer itself is set to, ordinary contrast,
// ordinary text, and movement left as the browser wants it.
type Appearance struct {
	// Theme is ThemeSystem, ThemeLight or ThemeDark.
	Theme string `json:"theme,omitempty"`
	// HighContrast darkens text and strengthens borders, for a screen in
	// sunlight or eyes that want the edges clearer.
	HighContrast bool `json:"high_contrast,omitempty"`
	// LargerText makes every word on the page bigger, without the browser's
	// own zoom, which people often don't know is there.
	LargerText bool `json:"larger_text,omitempty"`
	// ReduceMotion stops the spinner and the bars from moving. The browser
	// can already ask for this; this says it for isoshelf alone.
	ReduceMotion bool `json:"reduce_motion,omitempty"`
}

// The themes. System is the default: it follows whatever the computer is set
// to, and changes with it.
const (
	ThemeSystem = ""
	ThemeLight  = "light"
	ThemeDark   = "dark"
)

// CleanTheme turns anything unexpected into ThemeSystem, so a hand-edited
// settings file can't leave the page in a state nothing in isoshelf knows how
// to draw.
func CleanTheme(theme string) string {
	switch theme {
	case ThemeLight, ThemeDark:
		return theme
	}
	return ThemeSystem
}
