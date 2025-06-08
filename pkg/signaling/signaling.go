package signaling

import (
	"crypto/rand"
	"slices"

	"sync"
	"time"

	"github.com/gofiber/fiber/v2/log"

	"github.com/cloudlink-omega/accounts/pkg/authorization"
	account_structs "github.com/cloudlink-omega/accounts/pkg/structs"
	backend "github.com/cloudlink-omega/backend/pkg/database"
	"github.com/cloudlink-omega/signaling/pkg/signaling/handlers"
	"github.com/cloudlink-omega/signaling/pkg/signaling/message"
	"github.com/cloudlink-omega/signaling/pkg/signaling/origin"
	"github.com/cloudlink-omega/signaling/pkg/signaling/session"
	"github.com/cloudlink-omega/signaling/pkg/structs"
	"github.com/cloudlink-omega/storage/pkg/types"
	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/oklog/ulid/v2"
	"gorm.io/gorm"

	"github.com/valyala/fasthttp"
)

type Server structs.Server

func Initialize(allowedorigins []string, turnonly bool, auth *authorization.Auth, db *gorm.DB, perform_upgrade bool, gamedb *backend.Database) *Server {
	s := &Server{
		AuthorizedOriginsStorage: origin.CompilePatterns(allowedorigins),
		TURNOnly:                 turnonly,
		Lock:                     &sync.RWMutex{},
		Relays:                   make(map[string]map[string]*structs.Relay),
		Lobbies:                  make(map[string]map[string]*structs.Lobby),
		GlobalPeerIDs:            make(map[string][]string),
		UninitializedPeers:       make(map[string][]*structs.Client),
		Authorization:            auth,
		DB:                       db,
		GamesDB:                  gamedb,
	}

	if turnonly {
		log.Info("TURN only mode enabled. Candidates that specify STUN will be ignored, and only TURN candidates will be relayed.")
	}

	if db != nil {
		if perform_upgrade {
			s.DB.AutoMigrate(
				&types.User{},
				&types.Developer{},
				&types.DeveloperGame{},
			)
		}
	}

	return s
}

func RunClient(state *Server, c *structs.Client) {
	for {
		clientMsg, err := message.Read(c)
		if err != nil {
			log.Errorf("Client %s read error: %s", c.InstanceID, err.Error())
			return
		}
		HandleMessage(state, c, clientMsg)
	}
}

func CloseClient(state *Server, c *structs.Client) {
	session.UpdateState((*structs.Server)(state), nil, c, -1)
	if game := state.GlobalPeerIDs[c.GameID]; game != nil {
		state.GlobalPeerIDs[c.GameID] = slices.Delete(state.GlobalPeerIDs[c.GameID], slices.Index(game, c.InstanceID), 1)
	}
}

// AuthorizedOrigins implements the CheckOrigin method of the websocket.Upgrader.
// This checks if the incoming request's origin is allowed to Connect to the server.
// The server will log if the origin is permitted or rejected.
func (s *Server) AuthorizedOrigins(r *fasthttp.Request) bool {
	log.Debugf("Origin: %s, Host: %s", r.Header.Peek("Origin"), r.Host())

	// Check if the origin is allowed
	result := origin.IsAllowed(string(r.Header.Peek("Origin")), s.AuthorizedOriginsStorage)

	// Logging
	if result {
		log.Debug("Origin permitted to Connect")
	} else {
		log.Debug("Origin was rejected during Connect")
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
		message.Message = "A Game ID (UGI) is required to connect to this endpoint."
		return message
	}

	if string(c.Request().Header.Peek("Origin")) == "" {
		var message = fiber.ErrTeapot
		message.Message = "This is a websocket endpoint. Please use a WebSocket client or the CL5 extension."
		return message
	}

	// Check if the origin is allowed
	if !s.AuthorizedOrigins(c.Request()) {
		var message = fiber.ErrForbidden
		message.Message = "This origin is not permitted to connect to this endpoint. Please contact the server administrator."
		return message
	}

	// IsWebSocketUpgrade returns true if the client
	// requested upgrade to the WebSocket protocol.
	if websocket.IsWebSocketUpgrade(c) {
		c.Locals("allowed", true)

		// Attempt to passthrough authorization claims to the handler
		if s.Authorization != nil && s.Authorization.ValidFromNormal(c) {
			c.Locals("claims", s.Authorization.GetNormalClaims(c))
		}

		return c.Next()
	}

	// The client did not request a WebSocket upgrade
	var message = fiber.ErrUpgradeRequired
	message.Message = "This endpoint requires a WebSocket upgrade (did you access this endpoint using HTTP?). Please use a WebSocket client or the CL5 extension."
	return message
}

func RegisterClient(state *Server, c *structs.Client) {
	state.Lock.Lock()
	defer state.Lock.Unlock()
	func(state *Server, c *structs.Client) {
		if state.UninitializedPeers[c.GameID] == nil {
			log.Infof("Game ID %s uninitialized peers storage has been created", c.GameID)
			state.UninitializedPeers[c.GameID] = make([]*structs.Client, 0)
		}

		if state.GlobalPeerIDs[c.GameID] == nil {
			log.Infof("Game ID %s global peer IDs storage has been created", c.GameID)
			state.GlobalPeerIDs[c.GameID] = make([]string, 0)
		}

		state.UninitializedPeers[c.GameID] = append(state.UninitializedPeers[c.GameID], c)
		state.GlobalPeerIDs[c.GameID] = append(state.GlobalPeerIDs[c.GameID], c.InstanceID)
	}(state, c)
}

// Handler is an HTTP handler that handles WebSocket Connections and relays messages.
//
// Given an HTTP request, this function will upgrade the Connection to a WebSocket Connection
// and start a new client session. The function will then read all incoming messages, decode
// them, validate them, and handle them accordingly.
func (s *Server) Handler(Conn *websocket.Conn) {
	client := &structs.Client{
		Conn:            Conn,
		InstanceID:      ulid.MustNew(ulid.Timestamp(time.Now()), rand.Reader).String() + "_" + Conn.Query("ugi"),
		Token:           Conn.Query("token"),
		TokenWasPresent: Conn.Query("token") != "",
		Lock:            &sync.Mutex{},
		State:           0,
		TransmitLock:    &sync.Mutex{},
		GameID:          Conn.Query("ugi"),
	}

	if Conn.Query("ugi") == "" {
		session.CloseWithViolationMessage(client, "No Game ID provided (missing UGI parameter)")
		return
	}

	// Check if UGI is valid
	if s.DB != nil {
		client.Game = s.GamesDB.GetGame(Conn.Query("ugi"))
		if client.Game == nil {
			session.CloseWithViolationMessage(client, "Invalid Game ID (UGI not found)")
			return
		}
	}

	// Try to authorize the session
	claims, ok := Conn.Locals("claims").(*account_structs.Claims)
	if ok && claims != nil {
		client.AuthedWithCookie = true
		client.InstanceID = claims.ULID + "_" + Conn.Query("ugi")
		client.Name = claims.Username
		if slices.Contains(s.GlobalPeerIDs[Conn.Query("ugi")], client.InstanceID) {
			log.Infof("Game ID %s client with ID %s already exists", Conn.Query("ugi"), client.InstanceID)
			session.CloseWithViolationMessage(client, "session already in use for this game")
			return
		}
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
