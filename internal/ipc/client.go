package ipc

import (
	"encoding/json"
	"net"
)

type Client struct {
	socketPath string
	conn       net.Conn
}

func NewClient(socketPath string) *Client {
	return &Client{socketPath: socketPath}
}

func (c *Client) Connect() error {
	conn, err := net.Dial("unix", c.socketPath)
	if err != nil {
		c.conn = nil
		return err
	}
	c.conn = conn
	return nil
}

func (c *Client) IsConnected() bool {
	return c.conn != nil
}

func (c *Client) Close() {
	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}
}

func (c *Client) CallRaw(cmd string, params interface{}) (json.RawMessage, error) {
	conn, err := net.Dial("unix", c.socketPath)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	p, _ := json.Marshal(params)
	msg := Message{
		Command: cmd,
		Params:  p,
	}

	raw, _ := json.Marshal(msg)
	if _, err := conn.Write(append(raw, '\n')); err != nil {
		return nil, err
	}

	dec := json.NewDecoder(conn)
	var resp json.RawMessage
	if err := dec.Decode(&resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *Client) Call(cmd string, params interface{}) error {
	_, err := c.CallRaw(cmd, params)
	return err
}

// Listen starts a loop for receiving events
func (c *Client) Listen(handler func(Message)) {
	if c.conn == nil {
		return
	}
	dec := json.NewDecoder(c.conn)
	for {
		var msg Message
		if err := dec.Decode(&msg); err != nil {
			c.conn = nil
			return
		}
		if msg.Event != "" {
			handler(msg)
		}
	}
}
