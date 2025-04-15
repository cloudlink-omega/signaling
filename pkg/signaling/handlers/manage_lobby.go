package handlers

import (
	"encoding/json"

	"github.com/cloudlink-omega/signaling/pkg/signaling/message"
	"github.com/cloudlink-omega/signaling/pkg/signaling/session"
	"github.com/cloudlink-omega/signaling/pkg/structs"
)

func Manage_Lobby(state *structs.Server, c *structs.Client, wsMsg structs.Packet) {
	if !c.Valid {
		message.Send(c, structs.Packet{Opcode: "WARNING", Payload: "unauthorized"})
		return
	}

	// Must be the lobby host to manage the lobby
	if c.State != 1 {
		message.Send(c, structs.Packet{Opcode: "WARNING", Payload: "unauthorized"})
		return
	}

	// Get lobby
	lobby := state.Lobbies[c.GameID][c.Lobby]

	// Try to parse the Payload into args
	var args structs.ManageLobbyArgs
	raw, err := json.Marshal(wsMsg.Payload)
	if err != nil {
		session.CloseWithViolationMessage(c, err.Error())
		return
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		session.CloseWithViolationMessage(c, err.Error())
		return
	}

	switch args.Method {
	case "lock":
		lobby.Locked = true
	case "unlock":
		lobby.Locked = false
	case "kick":
		id, ok := args.Args.(string)
		if !ok {
			message.Send(c, structs.Packet{Opcode: "MANAGE_ACK", Payload: "type error: argument (peer id) should be a string"})
			return
		}

		// Get the client to kick
		client := session.Get(lobby.Clients, id)
		if client == nil {
			message.Send(c, structs.Packet{Opcode: "MANAGE_ACK", Payload: "no peer found"})
			return
		}

		// Kick the client
		session.CloseWithWarningMessage(client, "You have been kicked from the lobby")
		message.Send(c, structs.Packet{Opcode: "MANAGE_ACK", Payload: "ok"})
	case "change_password":
		newPassword, ok := args.Args.(string)
		if !ok {
			message.Send(c, structs.Packet{Opcode: "MANAGE_ACK", Payload: "type error: argument (new password) should be a string"})
			return
		}

		lobby.Password = newPassword
		message.Send(c, structs.Packet{Opcode: "MANAGE_ACK", Payload: "ok"})
	case "change_max_players":
		maxPlayers, ok := args.Args.(int64)
		if !ok {
			message.Send(c, structs.Packet{Opcode: "MANAGE_ACK", Payload: "type error: argument (max players) should be an integer"})
			return
		}

		if maxPlayers < -1 {
			message.Send(c, structs.Packet{Opcode: "MANAGE_ACK", Payload: "value error: argument (max players) should at least be -1 (unlimited), greater than or equal to than the current number of peers in the lobby"})
			return
		}

		// Don't update the size to be smaller than the current size (ignore if setting to unlimited)
		if maxPlayers != -1 && len(lobby.Clients) > int(maxPlayers) {
			message.Send(c, structs.Packet{Opcode: "MANAGE_ACK", Payload: "value error: new size is smaller than the current number of peers in the lobby"})
		}

		lobby.MaxPlayers = int64(maxPlayers)
		message.Send(c, structs.Packet{Opcode: "MANAGE_ACK", Payload: "ok"})
	}
}
