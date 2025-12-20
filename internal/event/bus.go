package event

import (
	"sync"
	"time"
)

type subscriber struct {
	name  string
	ch    chan Event
	drops uint64
}

// DropInfo reports dropped events for a subscriber.
type DropInfo struct {
	Name  string
	Count uint64
}

// Snapshot provides a point-in-time view for late-joining clients.
type Snapshot struct {
	State       State  `json:"state"`
	Hotkey      string `json:"hotkey"`
	LastEventID uint64 `json:"last_event_id"`
}

// Bus fan-outs events to subscribers without blocking publishers.
type Bus struct {
	mu sync.RWMutex

	subs map[string]*subscriber

	nextID      uint64
	lastState   State
	lastHotkey  string
	lastEventID uint64
}

// NewBus creates a new event bus instance.
func NewBus() *Bus {
	return &Bus{
		subs: make(map[string]*subscriber),
	}
}

// Subscribe registers a new subscriber with a bounded buffer.
func (b *Bus) Subscribe(name string, buf int) (<-chan Event, func()) {
	if buf <= 0 {
		buf = 1
	}

	ch := make(chan Event, buf)
	sub := &subscriber{name: name, ch: ch}

	b.mu.Lock()
	if old, ok := b.subs[name]; ok {
		close(old.ch)
	}
	b.subs[name] = sub
	b.mu.Unlock()

	cancel := func() {
		b.mu.Lock()
		if cur, ok := b.subs[name]; ok && cur == sub {
			delete(b.subs, name)
			close(ch)
		}
		b.mu.Unlock()
	}
	return ch, cancel
}

// Publish sends an event to all subscribers without blocking.
func (b *Bus) Publish(e Event) {
	if e.Time.IsZero() {
		e.Time = time.Now()
	}

	b.mu.Lock()
	b.nextID++
	e.ID = b.nextID
	b.lastEventID = e.ID

	if e.State != "" {
		b.lastState = e.State
	}
	if e.Hotkey != "" {
		b.lastHotkey = e.Hotkey
	}

	for _, sub := range b.subs {
		select {
		case sub.ch <- e:
		default:
			sub.drops++
		}
	}
	b.mu.Unlock()
}

// Snapshot returns the latest known state for reconciliation.
func (b *Bus) Snapshot() Snapshot {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return Snapshot{
		State:       b.lastState,
		Hotkey:      b.lastHotkey,
		LastEventID: b.lastEventID,
	}
}

// DrainDrops returns and resets drop counters for all subscribers.
func (b *Bus) DrainDrops() []DropInfo {
	b.mu.Lock()
	defer b.mu.Unlock()

	var out []DropInfo
	for _, sub := range b.subs {
		if sub.drops == 0 {
			continue
		}
		out = append(out, DropInfo{Name: sub.name, Count: sub.drops})
		sub.drops = 0
	}
	return out
}
