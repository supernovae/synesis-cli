package clipboard

import (
	"testing"
)

func TestCopyAndPaste(t *testing.T) {
	text := "test clipboard content " + t.Name()

	// Test Copy
	err := Copy(text)
	if err != nil {
		t.Skipf("clipboard not available on this platform: %v", err)
	}

	// Test Paste
	pasted, err := Paste()
	if err != nil {
		t.Fatalf("paste failed: %v", err)
	}

	if pasted != text {
		t.Errorf("paste returned different text:\n  expected: %q\n  got: %q", text, pasted)
	}
}

func TestPasteEmpty(t *testing.T) {
	// Test Paste when clipboard might be empty
	_, err := Paste()
	// We don't fail here - some platforms may have empty clipboard
	if err != nil {
		t.Logf("paste failed (may be empty): %v", err)
	}
}

func TestCopySpecialCharacters(t *testing.T) {
	tests := []string{
		"line1\nline2",
		"tabs\tand\tspaces",
		"unicode: \u00e9\u00e0\u00f1",
		"quotes: \"double\" and 'single'",
		"newlines\nand\r\nreturns",
	}

	for _, text := range tests {
		t.Run(text, func(t *testing.T) {
			err := Copy(text)
			if err != nil {
				t.Skipf("clipboard not available: %v", err)
			}

			pasted, err := Paste()
			if err != nil {
				t.Skipf("paste failed: %v", err)
			}

			if pasted != text {
				t.Errorf("mismatch:\n  expected: %q\n  got: %q", text, pasted)
			}
		})
	}
}
