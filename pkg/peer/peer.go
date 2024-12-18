package peer

import (
	"log"

	"github.com/goccy/go-json"

	"github.com/cloudlink-omega/signaling/pkg/manager"
	"github.com/cloudlink-omega/signaling/pkg/signaling/message"
	"github.com/cloudlink-omega/signaling/pkg/structs"
	"github.com/oklog/ulid/v2"
	"github.com/pion/webrtc/v4"
)

type Relay struct {
	*structs.Relay
}

func Spawn(s *structs.Server, ugi ulid.ULID, lobby string, peer *structs.Client) *structs.Relay {

	// Prepare the configuration
	policy := webrtc.ICETransportPolicyAll
	if s.TURNOnly {
		policy = webrtc.ICETransportPolicyRelay
	}

	// Build the configuration
	config := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs:           []string{"turn:vpn.mikedev101.cc:5349", "turn:vpn.mikedev101.cc:3478", "turn:freeturn.net:5349", "turn:freeturn.net:3478"},
				Username:       "free",
				Credential:     "free",
				CredentialType: webrtc.ICECredentialTypePassword,
			},
		},
		ICETransportPolicy: policy,
	}

	// Add STUN servers if not TURN only
	if !s.TURNOnly {
		config.ICEServers = append(config.ICEServers, webrtc.ICEServer{
			URLs: []string{"stun:vpn.mikedev101.cc:5349", "stun:vpn.mikedev101.cc:3478", "stun:stun.l.google.com:19302", "stun:freeturn.net:3478", "stun:freeturn.net:5349"},
		})
	}

	// Create a new RTCPeerConnection
	peerConnection, err := webrtc.NewPeerConnection(config)
	if err != nil {
		panic(err)
	}

	// Create a new relay peer
	relay := &structs.Relay{
		Server:           s,
		Conn:             peerConnection,
		Lobby:            lobby,
		UGI:              ugi,
		RequestShutdown:  make(chan bool),
		ShutdownComplete: make(chan bool),
		Running:          true,
		Peer:             peer,
		Channels:         make(map[string]*webrtc.DataChannel),
	}

	log.Printf("Relay [peer: %s, game: %s, lobby: %s] starting up...", relay.Peer.ULID.String(), relay.UGI.String(), relay.Lobby)

	// Create the default data channel
	yes := true
	zero := uint16(0)
	protocol := "clomega"
	relay.Channels["default"], err = peerConnection.CreateDataChannel("default", &webrtc.DataChannelInit{
		Negotiated: &yes,
		ID:         &zero,
		Ordered:    &yes,
		Protocol:   &protocol,
	})
	if err != nil {
		panic(err)
	} else {
		channelhandler(relay, relay.Channels["default"])
	}

	// Begin running the peer in the background
	go func() {

		// Do stuff while the peer is running. The peer might close at any time, but it needs to wait until the shutdown signal is received to fully close.
		handler(relay)

		// Keep running until the shutdown signal is received
		<-relay.RequestShutdown

		// Shutdown the peer if it is running
		if relay.Running {
			relay.Conn.Close()
			log.Printf("Relay [peer: %s, game: %s, lobby: %s] shutting down...", relay.Peer.ULID.String(), relay.UGI.String(), relay.Lobby)
		}

		// Send the shutdown complete signal
		relay.ShutdownComplete <- true
	}()

	return relay
}

func MakeOffer(r *structs.Relay) *webrtc.SessionDescription {

	// Create an offer.
	offer, offer_err := r.Conn.CreateOffer(&webrtc.OfferOptions{})
	if offer_err != nil {
		panic(offer_err)
	}

	// Set the local description.
	if err := r.Conn.SetLocalDescription(offer); err != nil {
		panic(err)
	}

	// Get the local description
	return r.Conn.LocalDescription()
}

func MakeAnswerFromOffer(r *structs.Relay, offer *webrtc.SessionDescription) *webrtc.SessionDescription {

	// Set the remote description.
	if err := r.Conn.SetRemoteDescription(*offer); err != nil {
		panic(err)
	}

	// Make answer
	answer, answer_err := r.Conn.CreateAnswer(&webrtc.AnswerOptions{})
	if answer_err != nil {
		panic(answer_err)
	}

	// Set the local description.
	if err := r.Conn.SetLocalDescription(answer); err != nil {
		panic(err)
	}

	// Get the local description
	return r.Conn.LocalDescription()
}

func HandleAnswer(r *structs.Relay, answer *webrtc.SessionDescription) {
	// Set the remote description.
	if err := r.Conn.SetRemoteDescription(*answer); err != nil {
		panic(err)
	}
}

func HandleIce(r *structs.Relay, ice *webrtc.ICECandidateInit) {
	// Add the ICE candidate.
	if err := r.Conn.AddICECandidate(*ice); err != nil {
		panic(err)
	}
}

// handler is a function that runs in the background and handles events related to the relay peer, such as connection state changes, messaging, and ICE candidates.
func handler(r *structs.Relay) {

	// Handle connection state changes
	r.Conn.OnConnectionStateChange(func(s webrtc.PeerConnectionState) {
		log.Printf("Relay [peer: %s, game: %s, lobby: %s] state changed to %s.", r.Peer.ULID.String(), r.UGI.String(), r.Lobby, s.String())

		switch s {
		case webrtc.PeerConnectionStateFailed:
			r.Running = false
			return

		case webrtc.PeerConnectionStateClosed:
			r.Running = false
			return
		}
	})

	// Handle ICE candidates
	r.Conn.OnICECandidate(func(c *webrtc.ICECandidate) {
		if c == nil {
			return
		}

		log.Printf("Relay [peer: %s, game: %s, lobby: %s] preparing ICE candidate: %s.", r.Peer.ULID.String(), r.UGI.String(), r.Lobby, c.ToJSON().Candidate)

		// Send the ICE candidate
		message.Code(
			r.Peer,
			"ICE",
			&structs.RelayOutboundIce{
				Type:     structs.DATA_CANDIDATE,
				Contents: c,
			},
			r.Lobby,
			&structs.PeerInfo{
				ID:   "relay",
				User: "relay",
			},
		)
	})

	r.Conn.OnDataChannel(func(d *webrtc.DataChannel) {
		channelhandler(r, d)
	})
}

func channelhandler(r *structs.Relay, d *webrtc.DataChannel) {

	d.OnError(func(err error) {
		log.Printf("Relay [peer: %s, game: %s, lobby: %s] data channel \"%s\" error: %s.", r.Peer.ULID.String(), r.UGI.String(), r.Lobby, d.Label(), err.Error())
	})

	d.OnOpen(func() {
		log.Printf("Relay [peer: %s, game: %s, lobby: %s] data channel \"%s\" open.", r.Peer.ULID.String(), r.UGI.String(), r.Lobby, d.Label())
	})

	d.OnClose(func() {
		log.Printf("Relay [peer: %s, game: %s, lobby: %s] data channel \"%s\" closed.", r.Peer.ULID.String(), r.UGI.String(), r.Lobby, d.Label())
	})

	d.OnMessage(func(msg webrtc.DataChannelMessage) {
		protocolhandler(r, d.Label(), string(msg.Data))
	})
}

func protocolhandler(r *structs.Relay, channel string, rawpacket string) {

	// Parse the message as JSON
	var packet structs.RelayPacket
	if err := json.Unmarshal([]byte(rawpacket), &packet); err != nil {
		log.Printf("Failed to parse message: %s", err.Error())
		return
	}

	// Handle the opcode
	switch packet.Opcode {

	case "G_MSG":
		relays := manager.WithoutRelay(manager.GetRelayPeers(r.Server, r.Lobby, r.UGI.String()), r)
		Broadcast(
			relays,
			channel,
			&structs.RelayPacket{
				Opcode:  "G_MSG",
				Payload: packet.Payload,
				Origin: &structs.PeerInfo{
					ID:   r.Peer.ULID.String(),
					User: r.Peer.Username,
				},
			},
		)

	case "G_VAR":
		relays := manager.WithoutRelay(manager.GetRelayPeers(r.Server, r.Lobby, r.UGI.String()), r)
		Broadcast(
			relays,
			channel,
			&structs.RelayPacket{
				Opcode:  "G_VAR",
				Payload: packet.Payload,
				Origin: &structs.PeerInfo{
					ID:   r.Peer.ULID.String(),
					User: r.Peer.Username,
				},
			},
		)

	case "G_LIST":
		relays := manager.WithoutRelay(manager.GetRelayPeers(r.Server, r.Lobby, r.UGI.String()), r)
		Broadcast(
			relays,
			channel,
			&structs.RelayPacket{
				Opcode:  "G_LIST",
				Payload: packet.Payload,
				Origin: &structs.PeerInfo{
					ID:   r.Peer.ULID.String(),
					User: r.Peer.Username,
				},
			},
		)

	case "P_MSG":
		if manager.VerifyRelayState(r, &packet) != nil {
			return
		}
		recipient := manager.GetRelay(r.Server, manager.GetByULID(r.Server, packet.Recipient))
		Send(
			recipient,
			channel,
			&structs.RelayPacket{
				Opcode:  "P_MSG",
				Payload: packet.Payload,
				Origin: &structs.PeerInfo{
					ID:   r.Peer.ULID.String(),
					User: r.Peer.Username,
				},
			},
		)

	case "P_VAR":
		if manager.VerifyRelayState(r, &packet) != nil {
			return
		}
		recipient := manager.GetRelay(r.Server, manager.GetByULID(r.Server, packet.Recipient))
		Send(
			recipient,
			channel,
			&structs.RelayPacket{
				Opcode:  "P_VAR",
				Payload: packet.Payload,
				Origin: &structs.PeerInfo{
					ID:   r.Peer.ULID.String(),
					User: r.Peer.Username,
				},
			},
		)

	case "P_LIST":
		if manager.VerifyRelayState(r, &packet) != nil {
			return
		}
		recipient := manager.GetRelay(r.Server, manager.GetByULID(r.Server, packet.Recipient))
		Send(
			recipient,
			channel,
			&structs.RelayPacket{
				Opcode:  "P_LIST",
				Payload: packet.Payload,
				Origin: &structs.PeerInfo{
					ID:   r.Peer.ULID.String(),
					User: r.Peer.Username,
				},
			},
		)

	default:
		log.Printf("Got unknown opcode: %s", packet.Opcode)
		Send(
			manager.GetRelay(r.Server, r.Peer),
			channel,
			&structs.RelayPacket{
				Opcode:  "WARN",
				Payload: "Unknown opcode: " + packet.Opcode,
			},
		)
	}
}
