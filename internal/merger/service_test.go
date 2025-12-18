package merger

import (
	"testing"
)

func TestMergerGolden(t *testing.T) {
	cfg := DefaultConfig()
	svc := NewMergerService(cfg)

	tests := []struct {
		name     string
		input    string
		wantComm string
		wantTent string
	}{
		{
			"Simple sentence",
			"Привет мир",
			"",
			"Привет мир",
		},
		{
			"Confirmed end",
			"Привет мир.",
			"Привет мир",
			"",
		},
		{
			"Russian punctuation",
			"Как дела?",
			"Как дела",
			"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			comm, tent := svc.Process(tt.input)
			if comm != tt.wantComm || tent != tt.wantTent {
				t.Errorf("Process(%q) = (%q, %q), want (%q, %q)", tt.input, comm, tent, tt.wantComm, tt.wantTent)
			}
			svc.Reset()
		})
	}
}

func TestLCSMerging(t *testing.T) {
	// Testing overlapping segments
	cfg := DefaultConfig()
	svc := NewMergerService(cfg)

	// Step 1: Fragment A
	svc.Process("Привет мой ")

	// Step 2: Fragment B (overlapping)
	comm, tent := svc.Process("мой дорогой друг")

	// The merger should identify "мой " as common and avoid duplication.
	// This is a simplified test case for the LCS logic.
	if comm != "" {
		t.Errorf("Expected empty committed initially, got %q", comm)
	}
	if tent == "Привет мой мой дорогой друг" {
		t.Errorf("Merger duplicated text: %q", tent)
	}
}
