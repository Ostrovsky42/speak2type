package merger

import "testing"

func TestMergerProcessAndFlush(t *testing.T) {
	svc := NewMergerService(DefaultConfig())

	committed, tentative := svc.Process("Привет мир")
	if committed != "" || tentative != "Привет мир" {
		t.Fatalf("first Process() = (%q, %q), want (%q, %q)", committed, tentative, "", "Привет мир")
	}

	flushed := svc.Flush()
	if flushed != "Привет мир" {
		t.Fatalf("Flush() = %q, want %q", flushed, "Привет мир")
	}
	if got := svc.GetCommitted(); got != "Привет мир" {
		t.Fatalf("GetCommitted() = %q, want %q", got, "Привет мир")
	}
}

func TestMergerCommitsAfterStableOverlap(t *testing.T) {
	svc := NewMergerService(Config{MinStability: 2})

	svc.Process("Привет мир")
	svc.Process("Привет мир")
	committed, tentative := svc.Process("Привет мир")

	if committed != "Привет мир" || tentative != "" {
		t.Fatalf("third Process() = (%q, %q), want (%q, %q)", committed, tentative, "Привет мир", "")
	}
}

func TestMergerCommitsPrefixThatFallsOutOfWindow(t *testing.T) {
	svc := NewMergerService(DefaultConfig())

	svc.Process("Привет мой")
	committed, tentative := svc.Process("мой дорогой друг")

	if committed != "Привет" {
		t.Fatalf("committed = %q, want %q", committed, "Привет")
	}
	if tentative != "мой дорогой друг" {
		t.Fatalf("tentative = %q, want %q", tentative, "мой дорогой друг")
	}
}

func TestMergerReset(t *testing.T) {
	svc := NewMergerService(DefaultConfig())
	svc.Process("Как дела?")
	svc.Reset()

	if got := svc.GetCommitted(); got != "" {
		t.Fatalf("GetCommitted() after Reset = %q, want empty", got)
	}
	if got := svc.Flush(); got != "" {
		t.Fatalf("Flush() after Reset = %q, want empty", got)
	}
}
