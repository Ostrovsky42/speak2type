package ipc

import (
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Message defines the JSON structure for IPC
type Message struct {
	Command string          `json:"command,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Event   string          `json:"event,omitempty"`
	Data    json.RawMessage `json:"data,omitempty"`
}

// StateInfo defines the state sent to clients
type StateInfo struct {
	State       string `json:"state"`
	Recording   bool   `json:"recording"`
	Language    string `json:"language"`
	Profile     string `json:"profile"`
	LastError   string `json:"last_error,omitempty"`
	FocusWindow string `json:"focus_window,omitempty"`
}

type Server struct {
	socketPath string
	listener   net.Listener
	conns      sync.Map // conn -> struct{}

	HandleFunc func(msg Message) (interface{}, error)
}

func NewServer(socketPath string) *Server {
	return &Server{
		socketPath: socketPath,
	}
}

func (s *Server) Start() error {
	// Ensure directory exists
	os.MkdirAll(filepath.Dir(s.socketPath), 0700)

	// Remove stale socket
	os.Remove(s.socketPath)

	l, err := net.Listen("unix", s.socketPath)
	if err != nil {
		return err
	}
	s.listener = l

	go s.acceptLoop()
	return nil
}

func (s *Server) Stop() {
	if s.listener != nil {
		s.listener.Close()
	}
	os.Remove(s.socketPath)
	s.conns.Range(func(key, value interface{}) bool {
		key.(net.Conn).Close()
		return true
	})
}

func (s *Server) acceptLoop() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return
		}
		s.conns.Store(conn, struct{}{})
		go s.handleConn(conn)
	}
}

func (s *Server) handleConn(conn net.Conn) {
	defer func() {
		conn.Close()
		s.conns.Delete(conn)
	}()

	dec := json.NewDecoder(conn)
	enc := json.NewEncoder(conn)

	for {
		var msg Message
		if err := dec.Decode(&msg); err != nil {
			return
		}

		if s.HandleFunc != nil {
			resp, err := s.HandleFunc(msg)
			if err != nil {
				enc.Encode(map[string]string{"error": err.Error()})
			} else {
				enc.Encode(resp)
			}
		}
	}
}

// Broadcast sends an event to all connected clients
func (s *Server) Broadcast(event string, data interface{}) {
	payload, _ := json.Marshal(data)
	msg := Message{
		Event: event,
		Data:  payload,
	}
	raw, _ := json.Marshal(msg)

	s.conns.Range(func(key, value interface{}) bool {
		conn := key.(net.Conn)
		go func(c net.Conn) {
			// Set a short deadline for the write to avoid blocking the goroutine forever
			c.SetWriteDeadline(time.Now().Add(100 * time.Millisecond))
			c.Write(append(raw, '\n'))
			// Clear deadline
			c.SetWriteDeadline(time.Time{})
		}(conn)
		return true
	})
}
