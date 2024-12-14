package handlers

import (
	"fmt"
	"log"

	"git.mikedev101.cc/MikeDEV/signaling/pkg/manager"
	"git.mikedev101.cc/MikeDEV/signaling/pkg/signaling/message"
	"git.mikedev101.cc/MikeDEV/signaling/pkg/structs"
)

func LOBBY_INFO(s *structs.Server, client *structs.Client, packet *structs.SignalPacket) {

	// Require the peer to be authorized
	if !client.AmIAuthorized() {
		err := message.Code(
			client,
			"CONFIG_REQUIRED",
			nil,
			packet.Listener,
			nil,
		)
		if err != nil {
			log.Printf("Send CONFIG_REQUIRED response to LOBBY_INFO opcode error: %s", err.Error())
		}
		return
	}

	// Assert that the payload is a string (lobby name)
	var lobby string
	switch packet.Payload.(type) {
	case string:
		lobby = packet.Payload.(string)
	default:
		message.Code(client, "VIOLATION", "Payload (lobby name) must be a string", packet.Listener, nil)
		client.Conn.Close()
		return
	}

	// Check if the requested lobby exists, or if it's the default lobby (which will never have lobby info)
	if lobby == "default" || !manager.DoesLobbyExist(s, lobby, client.UGI.String()) {
		message.Code(client, "LOBBY_NOTFOUND", nil, packet.Listener, nil)
		return
	}

	// Read lobby settings/state
	log.Printf("Getting lobby %s settings...", lobby)
	settings := manager.GetLobbySettings(s, lobby, client.UGI.String())

	// Check if the lobby is currently awaiting peer-based reclaim
	if settings.ReclaimInProgress {
		log.Printf("Lobby %s is currently hostless and awaiting peer-based reclaim", lobby)
		message.Code(
			client,
			"LOBBY_RECLAIM",
			nil,
			packet.Listener,
			nil,
		)
		return
	}

	// Retrieve the current lobby host
	log.Printf("Getting lobby %s host...", lobby)
	host, err := manager.GetLobbyHost(s, lobby, client.UGI.String())
	if err != nil {
		log.Printf("Get lobby host error: %s", err.Error())
		return
	}
	if host == nil {
		panic(fmt.Sprintf("No host assigned to lobby %s (nil value error) despite hostless flag being false. How did we get here?", lobby))
	}

	// Get a count of all members in the lobby - subtract 1 for the host
	log.Printf("Getting lobby %s members...", lobby)
	members := len(manager.GetLobbyPeers(s, lobby, client.UGI.String())) - 1

	// Send the reply
	message.Code(
		client,
		"LOBBY_INFO",
		&structs.LobbyInfo{
			LobbyHostID:       host.ULID.String(),
			LobbyHostUsername: host.Username,
			MaximumPeers:      manager.GetLobbySettings(s, lobby, client.UGI.String()).MaximumPeers,
			CurrentPeers:      members,
			PasswordRequired:  settings.Password != "",
			Reclaimable:       settings.AllowHostReclaim,
		},
		packet.Listener,
		nil,
	)
}
