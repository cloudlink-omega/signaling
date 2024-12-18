package handlers

import (
	"log"

	"github.com/cloudlink-omega/signaling/pkg/manager"
	"github.com/cloudlink-omega/signaling/pkg/signaling/message"
	"github.com/cloudlink-omega/signaling/pkg/structs"
)

// UNLOCK handles the UNLOCK opcode, which is used to unlock a lobby that is
// currently locked. The packet payload is empty, and the response
// payload is a structs.SignalPacket with the opcode set to "ACK_UNLOCK" and the
// payload containing a nil value. The response packet will be sent to the client
// that sent the packet.
func UNLOCK(s *structs.Server, client *structs.Client, packet *structs.SignalPacket) {

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
			log.Printf("Send CONFIG_REQUIRED response to UNLOCK opcode error: %s", err.Error())
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
			log.Printf("Send CONFIG_REQUIRED response to UNLOCK opcode error: %s", err.Error())
		}
		return
	}

	// Read lobby settings
	settings := manager.GetLobbySettings(s, client.Lobby, client.UGI.String())

	// Check if the lobby is already unlocked
	if !settings.Locked {
		err := message.Code(
			client,
			"ALREADY_UNLOCKED",
			nil,
			packet.Listener,
			nil,
		)
		if err != nil {
			log.Printf("Send ALREADY_UNLOCKED response to UNLOCK opcode error: %s", err.Error())
		}
		return
	}

	// Unlock the lobby
	settings.Locked = false
	manager.SetLobbySettings(s, client.Lobby, client.UGI.String(), settings)

	// Tell the host that the lobby was unlocked
	err := message.Code(
		client,
		"ACK_UNLOCK",
		nil,
		packet.Listener,
		nil,
	)
	if err != nil {
		log.Printf("Send ACK_LOCK response to UNLOCK opcode error: %s", err.Error())
	}
}
