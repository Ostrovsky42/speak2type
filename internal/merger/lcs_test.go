package merger

import (
	"strings"
	"testing"
)

func TestComputeOverlap(t *testing.T) {
	tests := []struct {
		name      string
		oldText   string
		newText   string
		expectLen int
	}{
		{
			name:      "Perfect Overlap",
			oldText:   "hello world",
			newText:   "hello world",
			expectLen: 2, // Both words
		},
		{
			name:      "Partial Suffix-Prefix",
			oldText:   "this is a test",
			newText:   "is a test of",
			expectLen: 3, // "is a test"
		},
		{
			name:      "No Overlap",
			oldText:   "hello world",
			newText:   "goodbye moon",
			expectLen: 0,
		},
		{
			name:      "Empty Old",
			oldText:   "",
			newText:   "hello",
			expectLen: 0,
		},
		{
			name:      "Empty New",
			oldText:   "hello",
			newText:   "",
			expectLen: 0,
		},
		{
			name:      "New is Substring of Old (End)",
			oldText:   "alpha beta gamma",
			newText:   "beta gamma",
			expectLen: 2,
		},
		{
			name:      "Correction (No strict overlap)",
			oldText:   "hello wrold",
			newText:   "hello world",
			expectLen: 1, // "hello" matches. "wrold" != "world". Overlap logic stops at mismatch if looking for suffix-prefix.
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oldTokens := strings.Fields(tt.oldText)
			newTokens := strings.Fields(tt.newText)

			got := ComputeOverlap(oldTokens, newTokens)
			if got != tt.expectLen {
				t.Errorf("ComputeOverlap(%q, %q) = %d; want %d", tt.oldText, tt.newText, got, tt.expectLen)
			}
		})
	}
}
