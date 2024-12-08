package chat

import (
	"errors"

	"github.com/gorilla/websocket"
)

const (
	opened = iota
	closed = iota
)

type Client struct {
	Name string
	Status int

	hub *Hub
	ws *websocket.Conn
	send chan []byte
}

func (c *Client) Close() {
	if c.Status != closed {
		c.ws.Close()
		close(c.send)
		c.hub.Unregister(c)
	}
	c.Status = closed
}

func (c *Client) Open(name string, ws *websocket.Conn, hub *Hub) {
	c.Name = name
	c.ws = ws
	c.hub = hub
	c.send = make(chan []byte, 256)
	c.Status = opened
	c.hub.Register(c)
	go c.receiving()
	go c.sending()
}

func (c *Client) Receive(p []byte) error {
	if c.Status == closed {
		return errors.New("client: Can not receive message. Client is closed")
	}
	c.send <- p
	return nil
}

func (c *Client) receiving() {
	defer c.Close()
	for p := range c.send {
		if err := c.ws.WriteMessage(websocket.BinaryMessage, p); err != nil {
			break
		}
	}
}

func (c *Client) sending() {
	defer c.Close()
	for {
		_, p, err := c.ws.ReadMessage()
		if err != nil {
			break
		}

		err = c.hub.Send(Message { c.Name, p })
		if err != nil {
			break
		}
	}
}
