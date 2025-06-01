package handlers

import (
	"crypto/rand"

	"github.com/cloudlink-omega/signaling/pkg/signaling/message"
	"github.com/cloudlink-omega/signaling/pkg/structs"
)

func Keepalive(state *structs.Server, c *structs.Client) {
	random_value := make([]byte, 16)
	rand.Read(random_value)
	message.Send(c, structs.Packet{Opcode: "KEEPALIVE_ACK", Payload: random_value})
}
