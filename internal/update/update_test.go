package update

import "testing"

func TestIsNewer(t *testing.T) {
	tests := []struct {
		latest, current string
		want            bool
	}{
		{"0.3.0", "0.2.0", true},
		{"v0.2.1", "0.2.0", true},
		{"0.2.0", "v0.2.0", false},
		{"0.1.9", "0.2.0", false},
		{"0.3.0-beta", "0.2.0", true},
	}
	for _, test := range tests {
		if got := isNewer(test.latest, test.current); got != test.want {
			t.Errorf("isNewer(%q, %q) = %v, want %v", test.latest, test.current, got, test.want)
		}
	}
}
