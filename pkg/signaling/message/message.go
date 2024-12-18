package message

import (
	"log"

	"github.com/cloudlink-omega/signaling/pkg/structs"
	"github.com/goccy/go-json"
	"github.com/gofiber/contrib/websocket"
)

func Send(client *structs.Client, message interface{}) error {
	if client == nil {
		log.Printf("Got a nil client when sending message: %v", message)
		return nil
	}

	// Marshal the message using go-json instead of interface/json
	bytes, err := json.Marshal(message)
	if err != nil {
		return err
	}

	// Send the message
	client.Mux.Lock()
	defer client.Mux.Unlock()
	return client.Conn.WriteMessage(websocket.TextMessage, bytes)
}

func Code(client *structs.Client, code string, message interface{}, listener string, origin *structs.PeerInfo) error {
	return Send(client, &structs.SignalPacket{Opcode: code, Payload: message, Listener: listener, Origin: origin})
}

func Broadcast(clients []*structs.Client, message interface{}) {
	for _, client := range clients {
		Send(client, message)
	}
}
