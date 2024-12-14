package handlers

import (
	"log"

	"git.mikedev101.cc/MikeDEV/signaling/pkg/manager"
	"git.mikedev101.cc/MikeDEV/signaling/pkg/signaling/message"
	"git.mikedev101.cc/MikeDEV/signaling/pkg/structs"
)

// LOCK handles the LOCK opcode, which is used to lock a lobby that is
// currently unlocked. The packet payload is empty, and the response
// payload is a structs.SignalPacket with the opcode set to "ACK_LOCK" and the
// payload containing a nil value. The response packet will be sent to the client
// that sent the packet.
func LOCK(s *structs.Server, client *structs.Client, packet *structs.SignalPacket) {

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
			log.Printf("Send CONFIG_REQUIRED response to LOCK opcode error: %s", err.Error())
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
			log.Printf("Send CONFIG_REQUIRED response to LOCK opcode error: %s", err.Error())
		}
		return
	}

	// Read lobby settings
	settings := manager.GetLobbySettings(s, client.Lobby, client.UGI.String())

	// Check if the lobby is already locked
	if settings.Locked {
		err := message.Code(
			client,
			"ALREADY_LOCKED",
			nil,
			packet.Listener,
			nil,
		)
		if err != nil {
			log.Printf("Send ALREADY_LOCKED response to LOCK opcode error: %s", err.Error())
		}
		return
	}

	// Lock the lobby
	settings.Locked = true
	manager.SetLobbySettings(s, client.Lobby, client.UGI.String(), settings)

	// Tell the host that the lobby was locked
	err := message.Code(
		client,
		"ACK_LOCK",
		nil,
		packet.Listener,
		nil,
	)
	if err != nil {
		log.Printf("Send ACK_LOCK response to LOCK opcode error: %s", err.Error())
	}
}
