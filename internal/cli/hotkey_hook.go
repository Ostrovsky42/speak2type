//go:build !nohook

package cli

import (
	"fmt"
	"runtime"
	"strings"
	"sync"
	"time"

	hook "github.com/robotn/gohook"
)

// HotkeyListener manages global hotkey events.
type HotkeyListener struct {
	triggerKey string
	onPress    func()
	stopChan   chan struct{}
	stopOnce   sync.Once
	wg         sync.WaitGroup
}

func NewHotkeyListener(key string, onPress func()) *HotkeyListener {
	return &HotkeyListener{
		triggerKey: key,
		onPress:    onPress,
		stopChan:   make(chan struct{}),
	}
}

// Start begins listening for the hotkey.
func (h *HotkeyListener) Start() {
	key := strings.ToLower(strings.TrimSpace(h.triggerKey))
	if key == "" {
		key = "f8"
	}

	if _, ok := hook.Keycode[key]; !ok {
		fmt.Printf("⚠️  Global Hotkey disabled: unknown key %q\n", h.triggerKey)
		return
	}

	fmt.Printf("⌨️  Global Hotkey Active: [%s]\n", key)

	invokeChan := make(chan struct{}, 1)

	// Execute user callback on a separate goroutine so we never block the hook loop.
	h.wg.Add(1)
	go func() {
		defer h.wg.Done()

		// gohook/libuiohook prefers staying on one OS thread.
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()

		// Keep callback fast and non-blocking to avoid event queue backpressure.
		const debounce = 250 * time.Millisecond
		var lastTrigger time.Time

		hook.Register(hook.KeyDown, []string{key}, func(e hook.Event) {
			now := time.Now()
			if !lastTrigger.IsZero() && now.Sub(lastTrigger) < debounce {
				return
			}
			lastTrigger = now
			select {
			case invokeChan <- struct{}{}:
			default:
			}
		})

		evChan := hook.Start()
		procDone := hook.Process(evChan)

		for {
			select {
			case <-h.stopChan:
				hook.End()
				<-procDone
				return
			case <-procDone:
				return
			}
		}
	}()

	h.wg.Add(1)
	go func() {
		defer h.wg.Done()

		for {
			select {
			case <-h.stopChan:
				return
			case <-invokeChan:
				fmt.Printf(" [Hotkey] Triggered: %s\n", key)
				h.onPress()
			}
		}
	}()
}

func (h *HotkeyListener) Stop() {
	h.stopOnce.Do(func() {
		close(h.stopChan)
	})
	h.wg.Wait()
}
