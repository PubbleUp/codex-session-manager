package version

import "testing"

func TestFormat(t *testing.T) {
	original := Version
	Version = "9.9.9-test"
	t.Cleanup(func() {
		Version = original
	})

	got := Format()
	want := "codex-session-manager 9.9.9-test\n"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}
