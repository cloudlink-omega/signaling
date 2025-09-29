package handlers

import (
	"encoding/json"

	"github.com/cloudlink-omega/signaling/pkg/signaling/message"
	"github.com/cloudlink-omega/signaling/pkg/signaling/session"
	"github.com/cloudlink-omega/signaling/pkg/structs"
)

func Join_Lobby(state *structs.Server, c *structs.Client, wsMsg structs.Packet) {
	if !c.Valid {
		message.Send(c, structs.Packet{Opcode: "WARNING", Payload: "unauthorized"})
		return
	}

	// Marshal the arguments into JSON
	var args structs.JoinLobbyArgs
	raw, err := json.Marshal(wsMsg.Payload)
	if err != nil {
		session.CloseWithViolationMessage(c, err.Error())
		return
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		session.CloseWithViolationMessage(c, err.Error())
		return
	}

	// Check if the lobby exists
	lobby := state.Lobbies[c.GameID][args.Name]
	if lobby == nil {
		message.Send(c, structs.Packet{Opcode: "JOIN_ACK", Payload: "not found"})
		return
	}

	// Check if the lobby is locked
	if lobby.Locked {
		message.Send(c, structs.Packet{Opcode: "JOIN_ACK", Payload: "locked"})
		return
	}

	// Check if the lobby is full (ignore if lobby.MaxPlayers == -1)
	if lobby.MaxPlayers != -1 && int64(len(lobby.Clients)) >= lobby.MaxPlayers {
		message.Send(c, structs.Packet{Opcode: "JOIN_ACK", Payload: "full"})
		return
	}

	// Check if the password is correct
	if lobby.Password != "" && lobby.Password != args.Password {
		message.Send(c, structs.Packet{Opcode: "JOIN_ACK", Payload: "password"})
		return
	}

	// Set the client as a member
	session.UpdateState(state, lobby, c, 2)
	message.Send(c, structs.Packet{Opcode: "JOIN_ACK", Payload: "ok"})

	if lobby.Host != nil {

		// Tell the peer about the current host
		message.Send(c, structs.Packet{Opcode: "NEW_HOST", Payload: structs.NewPeer{
			UserID:     lobby.Host.UserID,
			InstanceID: lobby.Host.InstanceID,
			PublicKey:  lobby.Host.PublicKey,
			Username:   lobby.Host.Name,
		}})

		// Tell the host and other peers about the new client
		message.Send(lobby.Host, structs.Packet{Opcode: "NEW_PEER", Payload: structs.NewPeer{
			UserID:     c.UserID,
			InstanceID: c.InstanceID,
			PublicKey:  c.PublicKey,
			Username:   c.Name,
		}})
	}

	// Tell existing members about the new peer
	message.Broadcast(session.Without(lobby.Clients, c), structs.Packet{Opcode: "PEER_JOIN", Payload: structs.NewPeer{
		UserID:     c.UserID,
		InstanceID: c.InstanceID,
		PublicKey:  c.PublicKey,
		Username:   c.Name,
	}})

	// Tell the peer about the relay (if present)
	if lobby.RelayEnabled {
		message.Send(c, structs.Packet{Opcode: "RELAY", Payload: lobby.RelayKey})
	}
}
