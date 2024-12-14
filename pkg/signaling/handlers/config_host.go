package handlers

import (
	"log"

	"git.mikedev101.cc/MikeDEV/signaling/pkg/manager"
	"git.mikedev101.cc/MikeDEV/signaling/pkg/peer"
	"git.mikedev101.cc/MikeDEV/signaling/pkg/signaling/message"
	"git.mikedev101.cc/MikeDEV/signaling/pkg/signaling/session"
	"git.mikedev101.cc/MikeDEV/signaling/pkg/structs"
	"github.com/goccy/go-json"
)

// CONFIG_HOST handles the CONFIG_HOST opcode, which is used to send configuration
// data about the peer to the server.
//
// The packet payload is a structs.HostConfigPacket, which contains data about the
// host, such as the lobby ID, whether the host is reclaimable, and the maximum
// number of peers allowed to connect to the host. It will also
// contain the public key of the peer if the peer has E2EE enabled.
//
// The response payload is a structs.SignalPacket with the opcode set to
// "ACK_HOST".
func CONFIG_HOST(s *structs.Server, client *structs.Client, rawpacket []byte, listener string) {

	// Don't start this handler if the client isn't authorized
	if !client.AmIAuthorized() {
		message.Code(
			client,
			"CONFIG_REQUIRED",
			nil,
			listener,
			nil,
		)
		return
	}

	// Prepare to transition to host mode
	if client.AmIPeer() {
		session.PrepareToChangeModesOrDisconnect(s, client)
		message.Code(
			client,
			"TRANSITION",
			"host",
			"",
			nil,
		)

		// Wait for the transition to finish before continuing
		<-client.TransitionDone

		client.ClearMode()
	}

	// Don't replay this handler if the client is already the host
	if client.AmIAHost() {
		message.Code(
			client,
			"ALREADY_HOST",
			nil,
			listener,
			nil,
		)
		return
	}

	// Read settings
	config := &structs.HostConfigPacket{}
	if err := json.Unmarshal(rawpacket, config); err != nil {
		log.Print("Parsing lobby settings error: ", err)
		message.Code(
			client,
			"VIOLATION",
			err,
			listener,
			nil,
		)
		session.Close(s, client)
		return
	}

	// Validate settings
	if err := s.PacketValidator.Struct(config); err != nil {
		log.Print("Validating lobby settings error: ", err)
		message.Code(
			client,
			"VIOLATION",
			err.Error(),
			listener,
			nil,
		)
		session.Close(s, client)
		return
	}

	OpenLobby(s, client, config, listener)
}

func OpenLobby(s *structs.Server, client *structs.Client, config *structs.HostConfigPacket, listener string) {

	// Create the lobby and add the client to it
	if manager.DoesLobbyExist(s, config.Payload.LobbyID, client.UGI.String()) {

		log.Printf("Lobby %s in game %s already exists", config.Payload.LobbyID, client.UGI)
		message.Code(
			client,
			"LOBBY_EXISTS",
			nil,
			listener,
			nil,
		)
		return
	}

	// Remove the client from the default lobby
	manager.RemoveClientFromLobby(s, "default", client.UGI.String(), client)

	// Create the lobby and configure it
	manager.AddClientToLobby(s, config.Payload.LobbyID, client.UGI.String(), client)
	manager.SetLobbySettings(s, config.Payload.LobbyID, client.UGI.String(), config.Payload)
	manager.SetLobbyHost(s, config.Payload.LobbyID, client.UGI.String(), client)

	// Set the client into host mode
	client.SetHostMode()

	// Set the client to the current lobby
	client.SetLobby(config.Payload.LobbyID)

	// Store the client public key (if specified)
	client.PublicKey = config.Payload.PublicKey

	// Notify other clients (that haven't joined a lobby) about the new host
	message.Broadcast(
		manager.WithoutPeer(
			manager.GetLobbyPeers(s, "default", client.UGI.String()),
			client,
		),
		&structs.SignalPacket{
			Opcode: "NEW_HOST",
			Payload: &structs.NewHostParams{
				ID:        client.ULID.String(),
				User:      client.Username,
				LobbyID:   config.Payload.LobbyID,
				PublicKey: client.PublicKey,
			},
		},
	)

	// Tell the client that it has been acknowledged
	message.Code(
		client,
		"ACK_HOST",
		nil,
		listener,
		nil,
	)

	// If the server-side relay was enabled for the lobby, spawn a new relay.
	if config.Payload.UseServerRelay {

		/*// Tell the client to anticipate a new relay connection
		message.Code(
			client,
			"ANTICIPATE",
			&structs.NewPeerParams{
				ID:   "relay",
				User: "relay",
			},
			"",
			nil,
		)*/

		// Spawn a new message relay
		relay := peer.Spawn(
			s,
			client.UGI,
			config.Payload.LobbyID,
			client,
		)

		// Store the relay
		manager.SetRelay(
			s,
			client,
			relay,
		)

		// Tell the client to discover a new relay connection
		message.Code(
			client,
			"DISCOVER",
			&structs.NewPeerParams{
				ID:   "relay",
				User: "relay",
			},
			"",
			nil,
		)

		/*// Generate an offer and send it
		message.Code(
			client,
			"MAKE_OFFER",
			&structs.RelayCandidate{
				Type:     structs.DATA_CANDIDATE,
				Contents: peer.MakeOffer(relay),
			},
			listener,
			&structs.NewPeerParams{
				ID:   "relay",
				User: "relay",
			},
		)*/
	}
}
