package handlers

import (
	"github.com/cloudlink-omega/signaling/pkg/signaling/message"
	"github.com/cloudlink-omega/signaling/pkg/structs"
)

func Find_Lobby(state *structs.Server, c *structs.Client, wsMsg structs.Packet) {
	if !c.Valid {
		message.Send(c, structs.Packet{Opcode: "WARNING", Payload: "unauthorized"})
		return
	}

	// Check if the lobby exists
	lobby := state.Lobbies[c.GameID][wsMsg.Payload.(string)]
	if lobby == nil {
		message.Send(c, structs.Packet{Opcode: "FIND_ACK", Payload: "not found"})
		return
	}

	// Return info about the lobby
	message.Send(c, structs.Packet{Opcode: "FIND_ACK", Payload: &structs.FindLobbyArgs{
		Host: structs.NewPeer{
			UserID:     lobby.Host.UserID,
			InstanceID: lobby.Host.InstanceID,
			PublicKey:  lobby.Host.PublicKey,
			Username:   lobby.Host.Name,
		},
		MaxPlayers:       lobby.MaxPlayers,
		CurrentPlayers:   uint64(len(lobby.Clients)),
		CurrentlyLocked:  lobby.Locked,
		PasswordRequired: lobby.Password != "",
	}})
}
