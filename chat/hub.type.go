package chat

import "errors"

type Hub struct {
	Status    int
	clients   map[*Client]bool
	broadcast chan Message
}

func (hub *Hub) Close() {
	if hub.Status != closed {
		for client := range hub.clients {
			hub.Unregister(client)
		}
		close(hub.broadcast)
	}
	hub.Status = closed
}

func (hub *Hub) Open() {
	hub.Status = opened
	hub.clients = make(map[*Client]bool)
	hub.broadcast = make(chan Message, 256)
	go hub.broadcasting()
}

func (hub *Hub) Send(mssg Message) error {
	if hub.Status == closed {
		return errors.New("hub: hub is closed")
	}
	hub.broadcast <- mssg
	return nil
}

func (hub *Hub) Register(client *Client) {
	hub.clients[client] = true
}

func (hub *Hub) Unregister(client *Client) {
	client.Close()
	delete(hub.clients, client)
}

func (hub *Hub) broadcasting() {
	for mssg := range hub.broadcast {
		for client := range hub.clients {
			str := mssg.Sender + ":" + string(mssg.Message)
			err := client.Receive([]byte(str))
			if err != nil {
				hub.Unregister(client)
			}
		}
	}
}