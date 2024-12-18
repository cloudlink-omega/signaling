package handlers

import (
	"log"

	"github.com/cloudlink-omega/signaling/pkg/manager"
	"github.com/cloudlink-omega/signaling/pkg/peer"
	"github.com/cloudlink-omega/signaling/pkg/signaling/message"
	"github.com/cloudlink-omega/signaling/pkg/structs"
	"github.com/goccy/go-json"
)

// Handles the MAKE_OFFER opcode. This function takes a client,
// an SDP offer, and forwards the offer to the desired peer. If
// the peer does not exist or isn't in the same lobby, the function sends the client a
// PEER_INVALID packet.
func MAKE_OFFER(s *structs.Server, client *structs.Client, packet *structs.SignalPacket, rawpacket []byte) {

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
			log.Printf("Send CONFIG_REQUIRED response to MAKE_OFFER opcode error: %s", err.Error())
		}
		return
	}

	// Read lobby settings. If the peer is the relay, handle the answer through the relay
	settings := manager.GetLobbySettings(s, client.Lobby, client.UGI.String())
	if packet.Recipient == "relay" {
		if !settings.UseServerRelay {
			return
		}

		// Read the raw packet as a relay packet
		reparsed := &structs.RelayCandidatePacket{}
		if err := json.Unmarshal(rawpacket, &reparsed); err != nil {
			log.Printf("Unmarshal relay MAKE_OFFER packet error: %s", err.Error())
			return
		}

		relay := manager.GetRelay(s, client)

		// The candidate type cannot be a voice candidate since we're a server, not a person.
		if reparsed.Payload.Type == structs.VOICE_CANDIDATE {
			log.Print("Handling MAKE_OFFER for relay peer can't be done: Got a voice candidate!")
			message.Code(
				client,
				"WARNING",
				"voice connections are not supported by the server relay",
				packet.Listener,
				nil,
			)
			return
		}

		answer := peer.MakeAnswerFromOffer(relay, reparsed.Payload.Contents)

		// Send the answer to the peer
		message.Code(
			client,
			"MAKE_ANSWER",
			&structs.RelayCandidate{
				Type:     structs.DATA_CANDIDATE,
				Contents: answer,
			},
			"",
			&structs.PeerInfo{
				ID:   "relay",
				User: "relay",
			},
		)
		return
	}

	// Check if the desired peer exists. If it does, get the peer's connection
	peer := manager.GetByULID(s, packet.Recipient)
	if peer == nil {
		log.Printf("Failed to get MAKE_OFFER peer as it doesn't exist: %s", packet.Recipient)
		return
	}

	// If the peer is nil or not in the lobby, send a PEER_INVALID packet
	if !manager.IsClientInLobby(s, client.Lobby, client.UGI.String(), peer) {
		err := message.Code(
			client,
			"PEER_INVALID",
			nil,
			packet.Listener,
			nil,
		)
		if err != nil {
			log.Printf("Send PEER_INVALID response to MAKE_OFFER opcode error: %s", err.Error())
		}
		return
	}

	// Relay the offer to the desired peer
	err := message.Code(
		peer,
		"MAKE_OFFER",
		packet.Payload,
		"",
		&structs.PeerInfo{
			ID:   client.ULID.String(),
			User: client.Username,
		},
	)
	if err != nil {
		log.Printf("Relay MAKE_OFFER opcode error: %s", err.Error())
	}

	// Tell the original client that the offer was relayed
	err = message.Code(
		client,
		"RELAY_OK",
		nil,
		packet.Listener,
		nil,
	)
	if err != nil {
		log.Printf("Send RELAY_OK response to MAKE_OFFER opcode error: %s", err.Error())
	}
}
