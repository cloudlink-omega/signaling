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

// CONFIG_PEER handles the CONFIG_PEER opcode, which is used to send configuration
// data about the peer to the server.
//
// The packet payload is a structs.PeerConfigPacket, which contains data about the
// selected lobby to join, and the password for the lobby (if any). It will also
// contain the public key of the peer if the peer has E2EE enabled.
//
// The response payload is a structs.SignalPacket with the opcode set to
// "ACK_PEER".
func CONFIG_PEER(s *structs.Server, client *structs.Client, rawpacket []byte, listener string) {

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

	// Prepare to transition to peer mode
	if client.AmIAHost() {
		session.PrepareToChangeModesOrDisconnect(s, client)
		message.Code(
			client,
			"TRANSITION",
			"peer",
			"",
			nil,
		)

		// Wait for the transition to finish before continuing
		<-client.TransitionDone

		client.ClearMode()
	}

	// Don't replay this handler if the client is already a peer
	if client.AmIPeer() {
		message.Code(
			client,
			"ALREADY_PEER",
			nil,
			listener,
			nil,
		)
		return
	}

	// Read parameters
	params := &structs.PeerConfigPacket{}
	if err := json.Unmarshal(rawpacket, params); err != nil {
		log.Print("Parsing lobby parameters error: ", err)
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

	// Validate parameters
	if err := s.PacketValidator.Struct(params); err != nil {
		log.Print("Validating lobby parameters error: ", err)
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

	JoinLobby(s, client, params, listener)
}

func JoinLobby(s *structs.Server, client *structs.Client, params *structs.PeerConfigPacket, listener string) {

	// Check if the requested lobby exists
	if !manager.DoesLobbyExist(s, params.Payload.LobbyID, client.UGI.String()) {

		log.Printf("Lobby %s in game %s doesn't exist", params.Payload.LobbyID, client.UGI)
		message.Code(
			client,
			"LOBBY_NOTFOUND",
			nil,
			listener,
			nil,
		)
		return
	}

	// Read lobby settings/state
	settings := manager.GetLobbySettings(s, params.Payload.LobbyID, client.UGI.String())

	// Check if the lobby is currently awaiting peer-based reclaim
	if settings.ReclaimInProgress {
		log.Printf("Lobby %s is currently hostless and awaiting peer-based reclaim", params.Payload.LobbyID)
		message.Code(
			client,
			"LOBBY_RECLAIM",
			nil,
			listener,
			nil,
		)
		return
	}

	// Get the count of current lobby members - subtract 1 for the host
	members := len(manager.GetLobbyPeers(s, params.Payload.LobbyID, client.UGI.String())) - 1

	// Check if the lobby is currently locked
	if settings.Locked {
		message.Code(
			client,
			"LOBBY_LOCKED",
			nil,
			listener,
			nil,
		)
		return
	}

	// Check if the lobby requires a password
	if settings.Password != "" {
		if params.Payload.Password == "" {
			message.Code(
				client,
				"PASSWORD_REQUIRED",
				nil,
				listener,
				nil,
			)
			return

		} else if params.Payload.Password != settings.Password {
			message.Code(
				client,
				"PASSWORD_FAIL",
				nil,
				listener,
				nil,
			)
			return

		} else {
			message.Code(
				client,
				"PASSWORD_ACK",
				nil,
				listener,
				nil,
			)
		}
	}

	// Check if the lobby is full
	if settings.MaximumPeers > 0 && members == settings.MaximumPeers {
		message.Code(
			client,
			"LOBBY_FULL",
			nil,
			listener,
			nil,
		)
		return
	}

	// Remove the client from the default lobby
	manager.RemoveClientFromLobby(s, "default", client.UGI.String(), client)

	// Create the lobby and configure it
	manager.AddClientToLobby(s, params.Payload.LobbyID, client.UGI.String(), client)

	// Set the client into peer mode
	client.SetPeerMode()

	// Set the client to the current lobby
	client.SetLobby(params.Payload.LobbyID)

	// Store the client public key (if specified)
	client.PublicKey = params.Payload.PublicKey

	// Get the current lobby host
	host, err := manager.GetLobbyHost(s, params.Payload.LobbyID, client.UGI.String())
	if err != nil {
		log.Printf("Get lobby host error: %s", err.Error())
		return
	}

	// Tell the host that a new peer has joined
	message.Code(
		host,
		"NEW_PEER",
		&structs.NewPeerParams{
			ID:        client.ULID.String(),
			User:      client.Username,
			PublicKey: client.PublicKey,
		},
		"",
		nil,
	)

	// Notify other peers in the lobby about the new member using the ANTICIPATE opcode.
	// This is a broadcast that prepares other peers to establish a connection with the new peer.
	only_peers := manager.GetLobbyPeers(s, params.Payload.LobbyID, client.UGI.String())
	only_peers = manager.WithoutPeer(only_peers, client)
	only_peers = manager.WithoutPeer(only_peers, host)
	message.Broadcast(
		only_peers,
		&structs.SignalPacket{
			Opcode: "ANTICIPATE",
			Payload: &structs.NewPeerParams{
				ID:        client.ULID.String(),
				User:      client.Username,
				PublicKey: client.PublicKey,
			},
		},
	)

	// Tell the client that it has been acknowledged
	message.Code(
		client,
		"ACK_PEER",
		nil,
		listener,
		nil,
	)

	// Tell the peer to expect a connection from the host
	message.Code(
		client,
		"ANTICIPATE",
		&structs.NewPeerParams{
			ID:        host.ULID.String(),
			User:      host.Username,
			PublicKey: host.PublicKey,
		},
		"",
		nil,
	)

	// Notify the new peer about other peers in the lobby using the DISCOVER opcode.
	// This tells the new peer to make connections with existing peers.
	existing := manager.GetLobbyPeers(s, params.Payload.LobbyID, client.UGI.String())
	existing = manager.WithoutPeer(existing, client)
	existing = manager.WithoutPeer(existing, host)
	for _, peer := range existing {
		message.Send(
			client,
			&structs.SignalPacket{
				Opcode: "DISCOVER",
				Payload: &structs.NewPeerParams{
					ID:        peer.ULID.String(),
					User:      peer.Username,
					PublicKey: peer.PublicKey,
				},
			},
		)
	}

	// If the server-side relay was enabled for the lobby, spawn a new relay.
	if settings.UseServerRelay {

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
			settings.LobbyID,
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
			&structs.PeerInfo{
				ID:   "relay",
				User: "relay",
			},
		)*/
	}
}
