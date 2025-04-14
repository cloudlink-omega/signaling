package signaling

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"slices"
	"sync"
	"time"

	"github.com/cloudlink-omega/signaling/pkg/signaling/message"
	"github.com/cloudlink-omega/signaling/pkg/signaling/origin"
	"github.com/cloudlink-omega/signaling/pkg/structs"
	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	peer "github.com/muka/peerjs-go"
	"github.com/oklog/ulid/v2"
	"github.com/pion/webrtc/v3"
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

func runClient(state *Server, c *structs.Client) {
	for {
		clientMsg, err := message.ReadMessage(c)
		if err != nil {
			log.Printf("WARNING: Client %s read error: %s", c.ID, err.Error())
			return
		}
		handleMessage(state, c, clientMsg)
	}
}

func closeClient(state *Server, c *structs.Client) {
	updateState(state, nil, c, -1)
}

// get returns the client with the given id from the given slice of clients.
// Returns nil if no client with the given id is found.
func get(peers []*structs.Client, id string) *structs.Client {
	for _, peer := range peers {
		if peer.ID == id {
			return peer
		}
	}
	return nil
}

// without returns a slice of clients that excludes the given client.
//
// It creates a new slice and filters out the given client. If the given client is
// not in the original slice, it returns the original slice unchanged.
func without(peers []*structs.Client, c *structs.Client) []*structs.Client {

	// Create a new slice
	copy := make([]*structs.Client, 0)

	// Scan through the original slice and filter out the client
	for _, peer := range peers {
		if peer != c {
			copy = append(copy, peer)
		}
	}
	return copy
}

// and returns a slice of clients that includes the given client.
//
// If the given client is already in the slice, it is returned unchanged.
// Otherwise, the given client is appended to the slice and the new slice is returned.
func and(peers []*structs.Client, c *structs.Client) []*structs.Client {

	// Create a copy
	copy := slices.Clone(peers)

	// Do nothing if it already contains the client
	if slices.Contains(copy, c) {
		return copy
	}

	// Add the client to the slice
	copy = append(copy, c)
	return copy
}

func spawnRelay(c *structs.Client, state *Server, lobby_name string) (*structs.Relay, error) {
	if state.Relays[c.GameID] != nil {
		return state.Relays[c.GameID][lobby_name], nil
	}

	config := peer.NewOptions()
	config.Configuration.ICEServers = []webrtc.ICEServer{
		{
			URLs: []string{"stun:vpn.mikedev101.cc:3478", "stun:vpn.mikedev101.cc:5349"},
		},
		{
			URLs:       []string{"turn:vpn.mikedev101.cc:5349", "turn:vpn.mikedev101.cc:3478"},
			Username:   "free",
			Credential: "free",
		},
	}
	config.Debug = 2

	// generate a random string to reduce collisions
	random_value := make([]byte, 5)
	rand.Read(random_value)
	base64_value := base64.StdEncoding.EncodeToString(random_value)
	relayid := fmt.Sprintf("relay_%s_%s-[%s]", c.GameID, lobby_name, base64_value)
	relayPeer, err := peer.NewPeer(relayid, config)
	if err != nil {
		log.Printf("Failed to create relay peer: %s", err)
		state.Lobbies[c.GameID][lobby_name].RelayEnabled = false
		return nil, err
	}

	relayObj := &structs.Relay{
		Handler:   relayPeer,
		Id:        relayid,
		Close:     make(chan bool),
		CloseDone: make(chan bool),
	}

	if state.Relays[c.GameID] == nil {
		log.Printf("Game %s relay storage has been created\n", c.GameID)
		state.Relays[c.GameID] = make(map[string]*structs.Relay)
	}

	if state.Relays[c.GameID][lobby_name] == nil {
		log.Printf("Game %s lobby %s relay storage has been created\n", c.GameID, lobby_name)
		state.Relays[c.GameID][lobby_name] = relayObj
	}

	log.Printf("Created relay peer for game %s lobby %s", c.GameID, lobby_name)

	go handleRelay(state, relayObj)
	return relayObj, nil
}

func handleRelay(_ *Server, r *structs.Relay) {
	p := r.Handler
	defer p.Destroy()

	p.On("Connection", func(data any) {
		Conn := data.(*peer.DataConnection)

		Conn.On("open", func(data any) {
			log.Printf("Peer %s Connected to relay", Conn.GetPeerID())
		})

		Conn.On("data", func(data any) {
			log.Printf("Received data from peer %s: %s", Conn.GetPeerID(), data)

			packet, ok := data.(map[string]any)
			if !ok {
				log.Printf("Packet casting failed: %s", data)
			}
			opcode, ok := packet["opcode"]
			if !ok {
				return
			}

			switch opcode {
			case "gmsg":
				// TODO: handle relay messages
				break
			}

		})

		Conn.On("close", func(data any) {
			log.Printf("Peer %s disConnected from relay", Conn.GetPeerID())
		})

		Conn.On("error", func(data any) {
			log.Printf("Peer %s error: %v", Conn.GetPeerID(), data)
		})
	})

	p.On("error", func(data any) {
		log.Printf("Relay peer error: %v", data)
	})

	p.On("close", func(data any) {
		log.Printf("Relay peer closed")
	})

	<-r.Close
	log.Printf("Relay peer got close signal.")
	r.CloseDone <- true
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

func validateToken(token string) bool {
	// TODO
	return token == "let me in"
}

func closeWithViolationMessage(c *structs.Client, message string) {
	c.Conn.WriteJSON(structs.Packet{Opcode: "VIOLATION", Payload: message})
	c.Conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseGoingAway, message), time.Now().Add(time.Second))
	c.Conn.Close()
}

func closeWithWarningMessage(c *structs.Client, message string) {
	c.Conn.WriteJSON(structs.Packet{Opcode: "WARNING", Payload: message})
	c.Conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseGoingAway, message), time.Now().Add(time.Second))
	c.Conn.Close()
}

func updateState(state *Server, lobby *structs.Lobby, c *structs.Client, newstate int8) {

	// Add to new state with lock. Both locks MUST be acquired and released at the same time!
	state.Lock.Lock()
	c.Lock.Lock()
	defer func(c *structs.Client, state *Server) {
		c.Lock.Unlock()
		state.Lock.Unlock()
	}(c, state)
	func(c *structs.Client, state *Server) {

		log.Printf("Peer %s was in state %d and is now in state %d\n", c.ID, c.State, newstate)

		if lobby == nil {
			// Try to find the lobby given the peer's lobby
			if c.Lobby != "" {
				lobby = state.Lobbies[c.GameID][c.Lobby]
			}
		}

		// First, remove from old global state
		switch c.State {

		case -1:
			log.Println("WARNING: Peer", c.ID, "last state was -1")

		// The client is uninitialized and is either being destroyed or joining a lobby
		case 0:

			// Remove the client from the uninitialized Clients
			state.UninitializedPeers[c.GameID] = without(state.UninitializedPeers[c.GameID], c)

		// The client was a host and the server needs to pick a new host
		case 1:
			if lobby != nil {
				if len(lobby.Clients) >= 1 {

					// Pick the next host
					newHost := lobby.Clients[0]
					log.Printf("Peer %s was in state %d and will become state 1\n", newHost.ID, newHost.State)
					newHost.State = 1
					lobby.Host = newHost
					lobby.Clients = without(lobby.Clients, newHost)
					message.WriteMessage(newHost, structs.Packet{Opcode: "TRANSITION", Payload: "host"})
					message.BroadcastMessage(lobby.Clients, structs.Packet{Opcode: "NEW_HOST", Payload: structs.NewPeer{
						UserID:    newHost.ID,
						PublicKey: newHost.PublicKey,
						Username:  newHost.Name,
					}})
				} else {
					log.Printf("Lobby %s has no more members, it will be destroyed", lobby.Name)
					lobby.Host = nil
				}
			}

		// The client was a member and needs to be removed
		case 2:
			if lobby != nil {

				// Remove the client from the lobby Clients
				lobby.Clients = without(lobby.Clients, c)
			}
		}

		// Then, update the client's state
		c.LastState = c.State

		if lobby == nil {
			c.Lobby = ""
		} else {
			c.Lobby = lobby.Name
		}

		c.State = newstate

		// Finally, add to new global state
		switch c.State {

		// Intended to finalize the destruction of the client
		case -1:
			// Remove the client from the uninitialized Clients
			state.UninitializedPeers[c.GameID] = without(state.UninitializedPeers[c.GameID], c)

			// Notify members the client is leaving
			if lobby != nil {

				// Does nothing if there are no peers
				message.BroadcastMessage(without(and(lobby.Clients, lobby.Host), c), structs.Packet{Opcode: "PEER_LEFT", Payload: c.ID})

				// Destroy the lobby if it's empty
				if c.LastState == 1 && lobby.Host == nil && len(lobby.Clients) == 0 {
					delete(state.Lobbies[c.GameID], lobby.Name)
					log.Printf("Lobby %s has been destroyed", lobby.Name)

					if lobby.RelayEnabled {
						state.Relays[c.GameID][lobby.Name].Close <- true
						<-state.Relays[c.GameID][lobby.Name].CloseDone
						log.Printf("Game ID %s lobby %s relay has been destroyed", c.GameID, c.Lobby)
						delete(state.Relays[c.GameID], lobby.Name)
					}

					message.BroadcastMessage(state.UninitializedPeers[c.GameID], structs.Packet{Opcode: "LOBBY_CLOSED", Payload: lobby.Name})
				}
			}

		// Client is now uninitialized
		case 0:
			state.UninitializedPeers[c.GameID] = and(state.UninitializedPeers[c.GameID], c)

		// Client needs to become a host
		case 1:

			// Get the old host
			oldHost := lobby.Host

			// Move the old host to the lobby Clients
			if oldHost != nil {
				log.Printf("Peer %s was in state %d and will become state 2\n", oldHost.ID, oldHost.State)
				oldHost.State = 2
				lobby.Clients = and(lobby.Clients, oldHost)
				message.WriteMessage(oldHost, structs.Packet{Opcode: "TRANSITION", Payload: "peer"})
			}

			// Set the new host
			lobby.Host = c
			message.WriteMessage(c, structs.Packet{Opcode: "TRANSITION", Payload: "host"})

		// Client needs to become a member
		case 2:
			lobby.Clients = and(lobby.Clients, c)
			message.WriteMessage(c, structs.Packet{Opcode: "TRANSITION", Payload: "peer"})
		}

		// Destroy all game storage if there are no lobbies
		log.Printf("Game ID %s has %d lobbies, %d uninitialized peers, and %d relays", c.GameID, len(state.Lobbies[c.GameID]), len(state.UninitializedPeers[c.GameID]), len(state.Relays[c.GameID]))
		if (len(state.UninitializedPeers[c.GameID]) == 0) && (len(state.Lobbies[c.GameID]) == 0) && (len(state.Relays[c.GameID]) == 0) {
			log.Printf("All Game ID %s storage has been destroyed due to no lobbies, relays, or uninitialized peers", c.GameID)
			delete(state.Lobbies, c.GameID)
			delete(state.UninitializedPeers, c.GameID)
			delete(state.Relays, c.GameID)
		}

	}(c, state)
}

func registerClient(state *Server, c *structs.Client) {
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
func (srv *Server) Handler(Conn *websocket.Conn) {

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
		closeWithViolationMessage(client, "no UGI provided")
		return
	}

	registerClient(srv, client)
	defer closeClient(srv, client)
	runClient(srv, client)
}

func handleMessage(state *Server, c *structs.Client, wsMsg structs.Packet) {
	switch wsMsg.Opcode {

	case "KEEPALIVE":
		random_value := make([]byte, 16)
		rand.Read(random_value)
		message.WriteMessage(c, structs.Packet{Opcode: "KEEPALIVE_ACK", Payload: random_value})

	case "INIT":
		if c.Valid {
			message.WriteMessage(c, structs.Packet{Opcode: "WARNING", Payload: "already authorized"})
			return
		}

		// Try to parse the Payload into args
		var args structs.InitArgs
		rawMessage, err := json.Marshal(wsMsg.Payload)
		if err != nil {
			closeWithViolationMessage(c, err.Error())
			return
		}
		if err := json.Unmarshal(rawMessage, &args); err != nil {
			closeWithViolationMessage(c, err.Error())
			return
		}

		// Require read lock to check token
		if !c.TokenWasPresent {
			c.Token = args.Token
		}
		if !validateToken(c.Token) {
			closeWithViolationMessage(c, "unauthorized")
			return
		}

		// Require write lock to set valid
		c.Lock.Lock()
		defer c.Lock.Unlock()
		c.Valid = true
		c.Name = args.Username
		c.PublicKey = args.PublicKey

		// Create game storage if it doesn't exist
		if state.Lobbies[c.GameID] == nil {
			log.Printf("Game ID %s lobby storage has been created", c.GameID)
			state.Lobbies[c.GameID] = make(map[string]*structs.Lobby)
		}

		// Return INIT_OK
		message.WriteMessage(c, structs.Packet{Opcode: "INIT_OK", Payload: structs.InitResponse{
			UserID:   c.ID,
			DevID:    "debug_id",
			Username: c.Name,
		}})

	case "LIST_LOBBIES":
		if !c.Valid {
			message.WriteMessage(c, structs.Packet{Opcode: "WARNING", Payload: "unauthorized"})
			return
		}

		// Return list of lobbies
		var lobbies []string
		for name := range state.Lobbies[c.GameID] {
			lobbies = append(lobbies, name)
		}

		if len(lobbies) == 0 {
			message.WriteMessage(c, structs.Packet{Opcode: "LIST_ACK", Payload: []string{}})
			return
		}

		message.WriteMessage(c, structs.Packet{Opcode: "LIST_ACK", Payload: lobbies})

	case "FIND_LOBBY":
		if !c.Valid {
			message.WriteMessage(c, structs.Packet{Opcode: "WARNING", Payload: "unauthorized"})
			return
		}

		// Check if the lobby exists
		lobby := state.Lobbies[c.GameID][wsMsg.Payload.(string)]
		if lobby == nil {
			message.WriteMessage(c, structs.Packet{Opcode: "FIND_ACK", Payload: "not found"})
			return
		}

		// Return info about the lobby
		message.WriteMessage(c, structs.Packet{Opcode: "FIND_ACK", Payload: &structs.FindLobbyArgs{
			Host: structs.NewPeer{
				UserID:    lobby.Host.ID,
				PublicKey: lobby.Host.PublicKey,
				Username:  lobby.Host.Name,
			},
			MaxPlayers:       lobby.MaxPlayers,
			CurrentPlayers:   uint64(len(lobby.Clients)),
			CurrentlyLocked:  lobby.Locked,
			PasswordRequired: lobby.Password != "",
		}})

	case "CREATE_LOBBY":
		if !c.Valid {
			message.WriteMessage(c, structs.Packet{Opcode: "WARNING", Payload: "unauthorized"})
			return
		}

		// Try to parse the Payload into args
		var args structs.CreateLobbyArgs
		raw, err := json.Marshal(wsMsg.Payload)
		if err != nil {
			closeWithViolationMessage(c, err.Error())
			return
		}
		if err := json.Unmarshal(raw, &args); err != nil {
			closeWithViolationMessage(c, err.Error())
			return
		}

		// Check if the lobby already exists
		if state.Lobbies[args.Name] != nil {
			message.WriteMessage(c, structs.Packet{Opcode: "CREATE_ACK", Payload: "exists"})
			return
		}

		// Create the lobby
		state.Lobbies[c.GameID][args.Name] = &structs.Lobby{
			Name:         args.Name,
			Lock:         &sync.RWMutex{},
			Password:     args.Password,
			MaxPlayers:   args.MaxPlayers,
			Locked:       args.Locked,
			RelayEnabled: args.EnableRelay,
			Clients:      make([]*structs.Client, 0),
		}
		log.Printf("Lobby %s was created and %s will become the first host", args.Name, c.ID)

		// Set the client as the host
		updateState(state, state.Lobbies[c.GameID][args.Name], c, 1)
		message.WriteMessage(c, structs.Packet{Opcode: "CREATE_ACK", Payload: "ok"})

		// Just tell the client that they are the host
		message.WriteMessage(c, structs.Packet{Opcode: "NEW_HOST", Payload: c.ID})

		// Tell other peers about the new lobby
		message.BroadcastMessage(state.UninitializedPeers[c.GameID], structs.Packet{Opcode: "NEW_LOBBY", Payload: args.Name})

		// Create a relay
		if args.EnableRelay {
			relay, err := spawnRelay(c, state, args.Name)
			if err != nil {
				return
			}
			state.Lobbies[c.GameID][args.Name].RelayKey = relay.Id
			message.WriteMessage(c, structs.Packet{Opcode: "RELAY", Payload: relay.Id})
		}

	case "JOIN_LOBBY":
		if !c.Valid {
			message.WriteMessage(c, structs.Packet{Opcode: "WARNING", Payload: "unauthorized"})
			return
		}

		// Marshal the arguments into JSON
		var args structs.JoinLobbyArgs
		raw, err := json.Marshal(wsMsg.Payload)
		if err != nil {
			closeWithViolationMessage(c, err.Error())
			return
		}
		if err := json.Unmarshal(raw, &args); err != nil {
			closeWithViolationMessage(c, err.Error())
			return
		}

		// Check if the lobby exists
		lobby := state.Lobbies[c.GameID][args.Name]
		if lobby == nil {
			message.WriteMessage(c, structs.Packet{Opcode: "JOIN_ACK", Payload: "not found"})
			return
		}

		// Check if the lobby is locked
		if lobby.Locked {
			message.WriteMessage(c, structs.Packet{Opcode: "JOIN_ACK", Payload: "locked"})
			return
		}

		// Check if the lobby is full (ignore if lobby.MaxPlayers == -1)
		if lobby.MaxPlayers != -1 && int64(len(lobby.Clients)) >= lobby.MaxPlayers {
			message.WriteMessage(c, structs.Packet{Opcode: "JOIN_ACK", Payload: "full"})
			return
		}

		// Check if the password is correct
		if lobby.Password != "" && lobby.Password != args.Password {
			message.WriteMessage(c, structs.Packet{Opcode: "JOIN_ACK", Payload: "password"})
			return
		}

		// Set the client as a member
		updateState(state, lobby, c, 2)
		message.WriteMessage(c, structs.Packet{Opcode: "JOIN_ACK", Payload: "ok"})

		// Tell the peer about the current host
		message.WriteMessage(c, structs.Packet{Opcode: "NEW_HOST", Payload: lobby.Host.ID})

		// Tell the host and other peers about the new client
		message.WriteMessage(lobby.Host, structs.Packet{Opcode: "NEW_PEER", Payload: structs.NewPeer{
			UserID:    c.ID,
			PublicKey: c.PublicKey,
			Username:  c.Name,
		}})
		message.BroadcastMessage(without(lobby.Clients, c), structs.Packet{Opcode: "PEER_JOIN", Payload: structs.NewPeer{
			UserID:    c.ID,
			PublicKey: c.PublicKey,
			Username:  c.Name,
		}})

		if lobby.RelayEnabled {
			message.WriteMessage(c, structs.Packet{Opcode: "RELAY", Payload: lobby.RelayKey})
		}

	case "MANAGE_LOBBY":
		if !c.Valid {
			message.WriteMessage(c, structs.Packet{Opcode: "WARNING", Payload: "unauthorized"})
			return
		}

		// Must be the lobby host to manage the lobby
		if c.State != 1 {
			message.WriteMessage(c, structs.Packet{Opcode: "WARNING", Payload: "unauthorized"})
			return
		}

		// Get lobby
		lobby := state.Lobbies[c.GameID][c.Lobby]

		// Try to parse the Payload into args
		var args structs.ManageLobbyArgs
		raw, err := json.Marshal(wsMsg.Payload)
		if err != nil {
			closeWithViolationMessage(c, err.Error())
			return
		}
		if err := json.Unmarshal(raw, &args); err != nil {
			closeWithViolationMessage(c, err.Error())
			return
		}

		switch args.Method {
		case "lock":
			lobby.Locked = true
		case "unlock":
			lobby.Locked = false
		case "kick":
			id, ok := args.Args.(string)
			if !ok {
				message.WriteMessage(c, structs.Packet{Opcode: "MANAGE_ACK", Payload: "type error: argument (peer id) should be a string"})
				return
			}

			// Get the client to kick
			client := get(lobby.Clients, id)
			if client == nil {
				message.WriteMessage(c, structs.Packet{Opcode: "MANAGE_ACK", Payload: "no peer found"})
				return
			}

			// Kick the client
			closeWithWarningMessage(client, "You have been kicked from the lobby")
			message.WriteMessage(c, structs.Packet{Opcode: "MANAGE_ACK", Payload: "ok"})
		case "change_password":
			newPassword, ok := args.Args.(string)
			if !ok {
				message.WriteMessage(c, structs.Packet{Opcode: "MANAGE_ACK", Payload: "type error: argument (new password) should be a string"})
				return
			}

			lobby.Password = newPassword
			message.WriteMessage(c, structs.Packet{Opcode: "MANAGE_ACK", Payload: "ok"})
		case "change_max_players":
			maxPlayers, ok := args.Args.(int64)
			if !ok {
				message.WriteMessage(c, structs.Packet{Opcode: "MANAGE_ACK", Payload: "type error: argument (max players) should be an integer"})
				return
			}

			if maxPlayers < -1 {
				message.WriteMessage(c, structs.Packet{Opcode: "MANAGE_ACK", Payload: "value error: argument (max players) should at least be -1 (unlimited), greater than or equal to than the current number of peers in the lobby"})
				return
			}

			// Don't update the size to be smaller than the current size (ignore if setting to unlimited)
			if maxPlayers != -1 && len(lobby.Clients) > int(maxPlayers) {
				message.WriteMessage(c, structs.Packet{Opcode: "MANAGE_ACK", Payload: "value error: new size is smaller than the current number of peers in the lobby"})
			}

			lobby.MaxPlayers = int64(maxPlayers)
			message.WriteMessage(c, structs.Packet{Opcode: "MANAGE_ACK", Payload: "ok"})
		}

	default:
		message.WriteMessage(c, structs.Packet{Opcode: "WARNING", Payload: "unknown or unimplemented opcode"})
	}
}
