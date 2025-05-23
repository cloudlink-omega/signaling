package handlers

import (
	"github.com/cloudlink-omega/signaling/pkg/signaling/message"
	"github.com/cloudlink-omega/signaling/pkg/structs"
)

func List_Lobbies(state *structs.Server, c *structs.Client) {
	if !c.Valid {
		message.Send(c, structs.Packet{Opcode: "WARNING", Payload: "unauthorized"})
		return
	}

	// Return list of lobbies
	var lobbies []string
	for name := range state.Lobbies[c.GameID] {
		lobbies = append(lobbies, name)
	}

	if len(lobbies) == 0 {
		message.Send(c, structs.Packet{Opcode: "LIST_ACK", Payload: []string{}})
		return
	}

	message.Send(c, structs.Packet{Opcode: "LIST_ACK", Payload: lobbies})
}
