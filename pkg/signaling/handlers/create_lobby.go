package handlers

import (
	"encoding/json"
	"sync"

	"github.com/gofiber/fiber/v2/log"

	"github.com/cloudlink-omega/signaling/pkg/signaling/message"
	"github.com/cloudlink-omega/signaling/pkg/signaling/relay"
	"github.com/cloudlink-omega/signaling/pkg/signaling/session"
	"github.com/cloudlink-omega/signaling/pkg/structs"
)

func Create_Lobby(state *structs.Server, c *structs.Client, wsMsg structs.Packet) {

	log.Debugf("$s $s $s", c.ID, c.GameID, wsMsg)

	if !c.Valid {
		message.Send(c, structs.Packet{Opcode: "WARNING", Payload: "unauthorized"})
		return
	}

	// Try to parse the Payload into args
	var args structs.CreateLobbyArgs
	raw, err := json.Marshal(wsMsg.Payload)
	if err != nil {
		session.CloseWithViolationMessage(c, err.Error())
		return
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		session.CloseWithViolationMessage(c, err.Error())
		return
	}

	// Check if the lobby already exists
	if state.Lobbies[c.GameID][args.Name] != nil {
		log.Infof("Lobby %s already exists", args.Name)
		message.Send(c, structs.Packet{Opcode: "CREATE_ACK", Payload: "exists"})
		return
	}

	// Create the lobby
	state.Lobbies[c.GameID][args.Name] = &structs.Lobby{
		Name:         args.Name,
		Lock:         &sync.RWMutex{},
		Password:     args.Password,
		MaxPlayers:   args.MaxPlayers,
		Locked:       args.Locked,
		RelayEnabled: args.EnableRelay,
		Clients:      make([]*structs.Client, 0),
	}
	log.Infof("Lobby %s was created and %s will become the first host", args.Name, c.ID)

	// Set the client as the host
	session.UpdateState(state, state.Lobbies[c.GameID][args.Name], c, 1)
	message.Send(c, structs.Packet{Opcode: "CREATE_ACK", Payload: "ok"})

	// Just tell the client that they are the host
	message.Send(c, structs.Packet{Opcode: "NEW_HOST", Payload: structs.NewPeer{
		UserID:    c.ID,
		PublicKey: c.PublicKey,
		Username:  c.Name,
	}})

	// Tell other peers about the new lobby
	message.Broadcast(state.UninitializedPeers[c.GameID], structs.Packet{Opcode: "NEW_LOBBY", Payload: args.Name})

	// Create a relay
	if args.EnableRelay {
		relay, err := relay.SpawnRelay(c, (*structs.Server)(state), args.Name)
		if err != nil {
			return
		}
		state.Lobbies[c.GameID][args.Name].RelayKey = relay.Id
		message.Send(c, structs.Packet{Opcode: "RELAY", Payload: relay.Id})
	}
}
