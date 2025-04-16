package signaling

import (
	"crypto/rand"

	"log"
	"sync"
	"time"

	"github.com/cloudlink-omega/signaling/pkg/signaling/handlers"
	"github.com/cloudlink-omega/signaling/pkg/signaling/message"
	"github.com/cloudlink-omega/signaling/pkg/signaling/origin"
	"github.com/cloudlink-omega/signaling/pkg/signaling/session"
	"github.com/cloudlink-omega/signaling/pkg/structs"
	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/oklog/ulid/v2"

	"github.com/valyala/fasthttp"
)

type Server structs.Server

func Initialize(allowedorigins []string, turnonly bool) *Server {
	s := &Server{
		AuthorizedOriginsStorage: origin.CompilePatterns(allowedorigins),
		TURNOnly:                 turnonly,
		Lock:                     &sync.RWMutex{},
		Relays:                   make(map[string]map[string]*structs.Relay),
		Lobbies:                  make(map[string]map[string]*structs.Lobby),
		UninitializedPeers:       make(map[string][]*structs.Client),
	}

	if turnonly {
		log.Print("TURN only mode enabled. Candidates that specify STUN will be ignored, and only TURN candidates will be relayed.")
	}

	return s
}

func RunClient(state *Server, c *structs.Client) {
	for {
		clientMsg, err := message.Read(c)
		if err != nil {
			log.Printf("WARNING: Client %s read error: %s", c.ID, err.Error())
			return
		}
		HandleMessage(state, c, clientMsg)
	}
}

func CloseClient(state *Server, c *structs.Client) {
	session.UpdateState((*structs.Server)(state), nil, c, -1)
}

// AuthorizedOrigins implements the CheckOrigin method of the websocket.Upgrader.
// This checks if the incoming request's origin is allowed to Connect to the server.
// The server will log if the origin is permitted or rejected.
func (s *Server) AuthorizedOrigins(r *fasthttp.Request) bool {
	log.Printf("Origin: %s, Host: %s", r.Header.Peek("Origin"), r.Host())

	// Check if the origin is allowed
	result := origin.IsAllowed(string(r.Header.Peek("Origin")), s.AuthorizedOriginsStorage)

	// Logging
	if result {
		log.Print("Origin permitted to Connect")
	} else {
		log.Print("Origin was rejected during Connect")
	}

	// TODO: cache the result to speed up future checks

	// Return the result
	return result
}

// Upgrader checks if the client requested a websocket upgrade, and if so,
// sets a local variable to true. If the client did not request a websocket
// upgrade, this middleware will return ErrUpgradeRequired. If the client
// is not allowed to Connect, this middleware will return ErrForbidden. If
// the client does not provide a UGI, this middleware will return ErrBadRequest.
func (s *Server) Upgrader(c *fiber.Ctx) error {

	// Check if UGI is provided
	if c.Query("ugi") == "" {
		var message = fiber.ErrBadRequest
		message.Message = "You attempted to access a WebSocket endpoint as a normal HTTP(s) request. Try using a WebSocket client or the CL5 extension."
		return message
	}

	if !s.AuthorizedOrigins(c.Request()) {
		var message = fiber.ErrForbidden
		message.Message = "This origin is not permitted to connect to this endpoint. Please contact the server administrator."
		return message
	}

	// IsWebSocketUpgrade returns true if the client
	// requested upgrade to the WebSocket protocol.
	if websocket.IsWebSocketUpgrade(c) {
		c.Locals("allowed", true)
		return c.Next()
	}

	var message = fiber.ErrUpgradeRequired
	message.Message = "This endpoint requires a WebSocket upgrade. Please use a WebSocket client or the CL5 extension."
	return message
}

func RegisterClient(state *Server, c *structs.Client) {
	state.Lock.Lock()

	if state.UninitializedPeers[c.GameID] == nil {
		log.Printf("Game ID %s uninitialized peers storage has been created", c.GameID)
		state.UninitializedPeers[c.GameID] = make([]*structs.Client, 0)
	}

	state.UninitializedPeers[c.GameID] = append(state.UninitializedPeers[c.GameID], c)
	state.Lock.Unlock()
}

// Handler is an HTTP handler that handles WebSocket Connections and relays messages.
//
// Given an HTTP request, this function will upgrade the Connection to a WebSocket Connection
// and start a new client session. The function will then read all incoming messages, decode
// them, validate them, and handle them accordingly.
func (s *Server) Handler(Conn *websocket.Conn) {

	client := &structs.Client{
		Conn:            Conn,
		ID:              ulid.MustNew(ulid.Timestamp(time.Now()), rand.Reader).String(),
		Token:           Conn.Query("token"),
		TokenWasPresent: Conn.Query("token") != "",
		Lock:            &sync.Mutex{},
		State:           0,
		TransmitLock:    &sync.Mutex{},
		GameID:          Conn.Query("ugi"),
	}

	if Conn.Query("ugi") == "" {
		session.CloseWithViolationMessage(client, "no UGI provided")
		return
	}

	RegisterClient(s, client)
	defer CloseClient(s, client)
	RunClient(s, client)
}

func HandleMessage(state *Server, c *structs.Client, wsMsg structs.Packet) {
	switch wsMsg.Opcode {

	case "KEEPALIVE":
		handlers.Keepalive((*structs.Server)(state), c)

	case "INIT":
		handlers.Init((*structs.Server)(state), c, wsMsg)

	case "LIST_LOBBIES":
		handlers.List_Lobbies((*structs.Server)(state), c)

	case "FIND_LOBBY":
		handlers.Find_Lobby((*structs.Server)(state), c, wsMsg)

	case "CREATE_LOBBY":
		handlers.Create_Lobby((*structs.Server)(state), c, wsMsg)

	case "JOIN_LOBBY":
		handlers.Join_Lobby((*structs.Server)(state), c, wsMsg)

	case "MANAGE_LOBBY":
		handlers.Manage_Lobby((*structs.Server)(state), c, wsMsg)

	default:
		message.Send(c, structs.Packet{Opcode: "WARNING", Payload: "unknown or unimplemented opcode"})
	}
}
