package message

import (
	"github.com/gofiber/fiber/v2/log"

	"github.com/cloudlink-omega/signaling/pkg/structs"
	"github.com/goccy/go-json"
)

func Send(c *structs.Client, wsMsg structs.Packet) {
	if c == nil {
		return
	}
	c.TransmitLock.Lock()
	defer c.TransmitLock.Unlock()
	c.Conn.WriteJSON(wsMsg)
}

func Read(c *structs.Client) (structs.Packet, error) {
	_, raw, err := c.Conn.ReadMessage()
	if err != nil {
		log.Errorf("Client read error:", err)
		return structs.Packet{}, err
	}

	var clientMsg structs.Packet
	if err := json.Unmarshal(raw, &clientMsg); err != nil {
		log.Errorf("Invalid message format")
		return structs.Packet{}, err
	}

	return clientMsg, nil
}

func Broadcast(peers []*structs.Client, wsMsg structs.Packet) {
	for _, peer := range peers {
		Send(peer, wsMsg)
	}
}
