package event

import "testing"

func TestBusDrainDrops(t *testing.T) {
	bus := NewBus()
	ch, cancel := bus.Subscribe("slow", 1)
	defer cancel()

	_ = ch

	bus.Publish(Event{Type: TypeAppStarted})
	bus.Publish(Event{Type: TypeAppStarted})
	bus.Publish(Event{Type: TypeAppStarted})

	drops := bus.DrainDrops()
	if len(drops) != 1 {
		t.Fatalf("expected 1 drop entry, got %d", len(drops))
	}
	if drops[0].Count != 2 {
		t.Fatalf("expected 2 drops, got %d", drops[0].Count)
	}
}

func TestBusSnapshot(t *testing.T) {
	bus := NewBus()
	bus.Publish(Event{Type: TypeAppStarted, State: StateIdle})
	bus.Publish(Event{Type: TypeHotkeyRegistered, Hotkey: "F8"})

	snap := bus.Snapshot()
	if snap.State != StateIdle {
		t.Fatalf("expected state %q, got %q", StateIdle, snap.State)
	}
	if snap.Hotkey != "F8" {
		t.Fatalf("expected hotkey F8, got %q", snap.Hotkey)
	}
	if snap.LastEventID == 0 {
		t.Fatalf("expected last event id to be set")
	}
}

func TestBusCancel(t *testing.T) {
	bus := NewBus()
	ch, cancel := bus.Subscribe("one", 1)
	cancel()

	select {
	case _, ok := <-ch:
		if ok {
			t.Fatalf("expected channel to be closed after cancel")
		}
	default:
		t.Fatalf("expected closed channel to read immediately")
	}
}
