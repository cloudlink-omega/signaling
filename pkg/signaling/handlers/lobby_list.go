package handlers

import (
	"log"

	"github.com/cloudlink-omega/signaling/pkg/manager"
	"github.com/cloudlink-omega/signaling/pkg/signaling/message"
	"github.com/cloudlink-omega/signaling/pkg/structs"
)

func LOBBY_LIST(s *structs.Server, client *structs.Client, packet *structs.SignalPacket) {

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
			log.Printf("Send CONFIG_REQUIRED response to LOBBY_LIST opcode error: %s", err.Error())
		}
		return
	}

	message.Code(
		client,
		"LOBBY_LIST",
		manager.GetAllLobbies(s, client.UGI.String()),
		packet.Listener,
		nil,
	)
}
