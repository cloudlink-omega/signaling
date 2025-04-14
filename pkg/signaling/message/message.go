package message

import (
	"log"

	"github.com/cloudlink-omega/signaling/pkg/structs"
	"github.com/goccy/go-json"
)

func WriteMessage(c *structs.Client, wsMsg structs.Packet) {
	if c == nil {
		return
	}
	c.TransmitLock.Lock()
	defer c.TransmitLock.Unlock()
	c.Conn.WriteJSON(wsMsg)
}

func ReadMessage(c *structs.Client) (structs.Packet, error) {
	_, raw, err := c.Conn.ReadMessage()
	if err != nil {
		log.Println("Client read error:", err)
		return structs.Packet{}, err
	}

	var clientMsg structs.Packet
	if err := json.Unmarshal(raw, &clientMsg); err != nil {
		log.Println("Invalid message format")
		return structs.Packet{}, err
	}
	return clientMsg, nil
}

func BroadcastMessage(peers []*structs.Client, wsMsg structs.Packet) {
	for _, peer := range peers {
		WriteMessage(peer, wsMsg)
	}
}
