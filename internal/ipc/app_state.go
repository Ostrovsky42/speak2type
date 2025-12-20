package ipc

// AppState provides a snapshot for late-joining UI clients.
type AppState struct {
	State       string `json:"state"`
	Hotkey      string `json:"hotkey"`
	LastEventID uint64 `json:"last_event_id"`
}
