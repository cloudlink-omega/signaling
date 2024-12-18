package signaling

import (
	"log"
	"sync"

	"github.com/cloudlink-omega/signaling/pkg/signaling/handlers"
	"github.com/cloudlink-omega/signaling/pkg/signaling/message"
	"github.com/cloudlink-omega/signaling/pkg/signaling/origin"
	"github.com/cloudlink-omega/signaling/pkg/signaling/session"
	"github.com/cloudlink-omega/signaling/pkg/structs"
	"github.com/go-playground/validator/v10"
	"github.com/goccy/go-json"
	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/oklog/ulid/v2"
	"github.com/valyala/fasthttp"
)

type Server structs.Server

func Initialize(allowedorigins []string, turnonly bool) *Server {
	s := &Server{
		AuthorizedOriginsStorage: origin.CompilePatterns(allowedorigins),
		Mux:                      &sync.RWMutex{},
		TURNOnly:                 turnonly,
		Games:                    &structs.GameStore{Mutex: sync.RWMutex{}, Games: make(map[string]*structs.Game)},
		Sessions:                 &structs.SessionStore{Mutex: sync.RWMutex{}, Sessions: make(map[string]*structs.Session)},
		Relays:                   make(map[*structs.Client]*structs.Relay),
		RelayLock:                &sync.RWMutex{},
		PacketValidator:          validator.New(validator.WithRequiredStructEnabled()),
		WebsocketConnCounter:     0,
	}

	if turnonly {
		log.Print("TURN only mode enabled. Candidates that specify STUN will be ignored, and only TURN candidates will be relayed.")
	}

	return s
}

// AuthorizedOrigins implements the CheckOrigin method of the websocket.Upgrader.
// This checks if the incoming request's origin is allowed to connect to the server.
// The server will log if the origin is permitted or rejected.
func (s *Server) AuthorizedOrigins(r *fasthttp.Request) bool {
	log.Printf("Origin: %s, Host: %s", r.Header.Peek("Origin"), r.Host())

	// Check if the origin is allowed
	result := origin.IsAllowed(string(r.Header.Peek("Origin")), s.AuthorizedOriginsStorage)

	// Logging
	if result {
		log.Print("Origin permitted to connect")
	} else {
		log.Print("Origin was rejected during connect")
	}

	// TODO: cache the result to speed up future checks

	// Return the result
	return result
}

// Upgrader checks if the client requested a websocket upgrade, and if so,
// sets a local variable to true. If the client did not request a websocket
// upgrade, this middleware will return ErrUpgradeRequired. If the client
// is not allowed to connect, this middleware will return ErrForbidden. If
// the client does not provide a UGI, this middleware will return ErrBadRequest.
func (s *Server) Upgrader(c *fiber.Ctx) error {

	// Check if UGI is provided
	if c.Query("ugi") == "" {
		return fiber.ErrBadRequest
	}

	if !s.AuthorizedOrigins(c.Request()) {
		return fiber.ErrForbidden
	}

	// IsWebSocketUpgrade returns true if the client
	// requested upgrade to the WebSocket protocol.
	if websocket.IsWebSocketUpgrade(c) {
		c.Locals("allowed", true)
		return c.Next()
	}

	return fiber.ErrUpgradeRequired
}

// Handler is an HTTP handler that handles WebSocket connections and relays messages.
//
// Given an HTTP request, this function will upgrade the connection to a WebSocket connection
// and start a new client session. The function will then read all incoming messages, decode
// them, validate them, and handle them accordingly.
func (srv *Server) Handler(conn *websocket.Conn) {
	// Cast server
	s := (*structs.Server)(srv)

	// Validate UGI
	ugi, err := ulid.Parse(conn.Query("ugi"))
	if err != nil {

		// Tell the client that the UGI is bad.
		if err := conn.WriteJSON(&structs.SignalPacket{
			Opcode:  "VIOLATION",
			Payload: "Unique Game Identifier (UGI) invalid or malformed",
		}); err != nil {
			log.Printf("Handle bad UGI error: %s", err.Error())
		}

		// Close the connection
		conn.Close()
		return
	}

	// Start session
	client := session.Open(s, conn, ugi)

	// Handle messages and close handler when disconnected
	defer session.Close(s, client)
	for {

		// Read packet
		_, rawpacket, err := conn.ReadMessage()
		if err != nil {
			if !(websocket.IsCloseError(err) || websocket.IsUnexpectedCloseError(err)) {
				log.Fatalf("WebSocket unhandled receive error: %s\n", err)
			}
			return
		}

		// Decode packet
		var packet *structs.SignalPacket
		if err := json.Unmarshal(rawpacket, &packet); err != nil {
			conn.WriteJSON(&structs.SignalPacket{
				Opcode:  "VIOLATION",
				Payload: "Packet decoding error",
			})
			return
		}

		// Validate the packet
		if err := s.PacketValidator.Struct(packet); err != nil {
			conn.WriteJSON(&structs.SignalPacket{
				Opcode:  "VIOLATION",
				Payload: err.Error(),
			})
			return
		}

		// Allow for concurrent packet processing
		go execute_packet(s, client, packet, rawpacket)
	}
}

func execute_packet(s *structs.Server, client *structs.Client, packet *structs.SignalPacket, rawpacket []byte) {
	// Handle opcodes accordingly.
	switch packet.Opcode {

	// Keep connection alive
	case "KEEPALIVE":
		message.Code(
			client,
			"KEEPALIVE",
			packet.Payload,
			packet.Listener,
			nil,
		)

	// Initializes the session.
	case "INIT":
		handlers.INIT(s, client, packet)

	// Shares metadata about the client, and returns metadata about the server.
	case "META":
		handlers.META(s, client, packet)

	// Makes the peer a lobby host.
	case "CONFIG_HOST":
		handlers.CONFIG_HOST(s, client, rawpacket, packet.Listener)

	// Makes the peer a lobby member.
	case "CONFIG_PEER":
		handlers.CONFIG_PEER(s, client, rawpacket, packet.Listener)

	// Relays SDP offer data.
	case "MAKE_OFFER":
		handlers.MAKE_OFFER(s, client, packet, rawpacket)

	// Relays SDP answer data.
	case "MAKE_ANSWER":
		handlers.MAKE_ANSWER(s, client, packet, rawpacket)

	// Relays SDP ICE data.
	case "ICE":
		handlers.ICE(s, client, packet, rawpacket)

	// Provides a list of all open lobbies to join.
	case "LOBBY_LIST":
		handlers.LOBBY_LIST(s, client, packet)

	// Provides information about a lobby.
	case "LOBBY_INFO":
		handlers.LOBBY_INFO(s, client, packet)

	// Prevents new peers from joining a lobby.
	case "LOCK":
		handlers.LOCK(s, client, packet)

	// Permits new peers to join a lobby.
	case "UNLOCK":
		handlers.UNLOCK(s, client, packet)

	// Modifies the maximum number of peers allowed in a lobby.
	case "SIZE":
		handlers.SIZE(s, client, packet)

	case "TRANSITION_ACK":
		client.TransitionDone <- true

	// Kicks a peer from a lobby.
	case "KICK":
		return

	// Allows a peer to initialize a voice call (fix for timing bug)
	case "DIAL":
		return

	// Allows a peer to decline an incoming voice call.
	case "DECLINE":
		return

	// Allows a peer to accept an incoming voice call.
	case "ACCEPT":
		return

	default:
		message.Code(
			client,
			"VIOLATION",
			"Unknown opcode",
			packet.Listener,
			nil,
		)
		return
	}
}
