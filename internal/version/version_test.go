package version

import "testing"

func TestCompare(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"22.3", "22.3", 0},
		{"22.4", "22.10", -1},
		{"25.2", "21.3", 1},
		{"260809", "260308", 1},
		{"56", "58", -1},
		{"3.0.3", "3.0.10", -1},
		{"23.0.4", "23.0", 1},
		{"23.0.4", "23-0-4", 0},
		{"007", "7", 0},
		{"22.0-SP1", "22.0", 1},
		{"44.20260916", "44.20260915.1", 1},
		{"23H2", "24H2", -1},
		{"1.0.1", "1.0.beta", 1},
		{"Beta", "alpha", 1},
		{"", "1", -1},
		{"12345678901234567890", "12345678901234567891", -1},
	}
	for _, tt := range tests {
		if got := Compare(tt.a, tt.b); got != tt.want {
			t.Errorf("Compare(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
		if got := Compare(tt.b, tt.a); got != -tt.want {
			t.Errorf("Compare(%q, %q) = %d, want %d", tt.b, tt.a, got, -tt.want)
		}
	}
}
