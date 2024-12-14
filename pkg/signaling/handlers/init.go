package handlers

import (
	"log"

	"git.mikedev101.cc/MikeDEV/signaling/pkg/signaling/message"
	"git.mikedev101.cc/MikeDEV/signaling/pkg/structs"
)

// INIT handles the INIT opcode, which is used to initialize a new connection to the signaling server.
//
// The packet payload is (currently) empty, and the response payload is a structs.SignalPacket with the opcode
// set to "INIT_OK" and the payload containing a structs.InitOK, which contains the user ID, game, and
// developer identifier. The response packet will be sent to the client that sent the packet.
func INIT(s *structs.Server, client *structs.Client, packet *structs.SignalPacket) {

	// If the peer is already authorized, send a SESSION_EXISTS opcode
	if client.AmIAuthorized() {
		err := message.Code(
			client,
			"SESSION_EXISTS",
			nil,
			packet.Listener,
			nil,
		)
		if err != nil {
			log.Printf("Send SESSION_EXISTS response to INIT opcode error: %s", err.Error())
		}
		return
	}

	/*
	   TODO: read the payload as a JWT,
	   read the username, user ID, and originating authentication server address, and then confirm with the auth server
	   that the JWT is valid. If not, return a
	   TOKEN_INVALID response.

	   Using the ugi query string parameter, we will need
	   to check with the game server that the UGI is valid
	   and to retrieve the game's name and developer ID.

	   Later on, we will also need to serve any game-specific
	   data to the client, such as public storage data.
	*/

	// TODO: read payload and do something with it

	// Let's just authorize the peer for now
	client.Username = packet.Payload.(string)
	client.StoreAuthorization("something")

	// Dummy values for now
	err := message.Code(
		client,
		"INIT_OK",
		&structs.InitOK{
			User:      client.Username,
			Id:        client.ULID.String(),
			SessionID: client.Session,
			Game:      "Testing",
			Developer: "Testing",
		},
		packet.Listener,
		nil,
	)
	if err != nil {
		log.Printf("Send response to INIT opcode error: %s", err.Error())
	}
}
