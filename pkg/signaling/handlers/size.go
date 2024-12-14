package handlers

import (
	"log"

	"git.mikedev101.cc/MikeDEV/signaling/pkg/manager"
	"git.mikedev101.cc/MikeDEV/signaling/pkg/signaling/message"
	"git.mikedev101.cc/MikeDEV/signaling/pkg/signaling/session"
	"git.mikedev101.cc/MikeDEV/signaling/pkg/structs"
)

func SIZE(s *structs.Server, client *structs.Client, packet *structs.SignalPacket) {

	// Require the peer to be in a lobby
	if !client.AmIInALobby() {
		err := message.Code(
			client,
			"CONFIG_REQUIRED",
			nil,
			packet.Listener,
			nil,
		)
		if err != nil {
			log.Printf("Send CONFIG_REQUIRED response to SIZE opcode error: %s", err.Error())
		}
		return
	}

	// If the peer is not the host, send a WARNING packet
	if client.AmIAHost() {
		err := message.Code(
			client,
			"WARNING",
			"Not the lobby host",
			packet.Listener,
			nil,
		)
		if err != nil {
			log.Printf("Send CONFIG_REQUIRED response to SIZE opcode error: %s", err.Error())
		}
		return
	}

	// Check if the payload is an int
	var size int
	switch packet.Payload.(type) {
	case int:
		size = packet.Payload.(int)
	default:
		err := message.Code(
			client,
			"VIOLATION",
			"Payload (lobby size) must be an integer",
			packet.Listener,
			nil,
		)
		if err != nil {
			log.Printf("Send VIOLATION response to SIZE opcode error: %s", err.Error())
		}
		session.Close(s, client)
		return
	}

	// Read lobby settings
	settings := manager.GetLobbySettings(s, client.Lobby, client.UGI.String())

	// Get a count of all members in the lobby - subtract 1 for the host
	log.Printf("Getting lobby %s members...", client.Lobby)
	members := len(manager.GetLobbyPeers(s, client.Lobby, client.UGI.String())) - 1

	// Don't allow the lobby to be resized smaller than the current number of members - Ignore if setting to zero, which means no limit.
	if size != 0 && size < members {
		err := message.Code(
			client,
			"WARNING",
			"Lobby size cannot be reduced to less than the current number of members",
			packet.Listener,
			nil,
		)
		if err != nil {
			log.Printf("Send WARNING response to SIZE opcode error: %s", err.Error())
		}
		return
	}

	// Update the lobby settings
	settings.MaximumPeers = size
	manager.SetLobbySettings(s, client.Lobby, client.UGI.String(), settings)

	// Tell the host that the lobby player limit was changed
	err := message.Code(
		client,
		"ACK_SIZE",
		nil,
		packet.Listener,
		nil,
	)
	if err != nil {
		log.Printf("Send ACK_SIZE response to SIZE opcode error: %s", err.Error())
	}
}
