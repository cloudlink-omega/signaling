package session

import (
	"log"
	"slices"
	"time"

	"github.com/cloudlink-omega/signaling/pkg/signaling/message"
	"github.com/cloudlink-omega/signaling/pkg/structs"
	"github.com/gofiber/contrib/websocket"
)

func DestroyLobby(state *structs.Server, lobby *structs.Lobby, c *structs.Client) {
	log.Println("Destroy Lobby was called!")

	if lobby != nil && c.LastState == 1 && lobby.Host == nil && len(lobby.Clients) == 0 {
		if lobby.RelayEnabled {
			state.Relays[c.GameID][lobby.Name].Close <- true
			<-state.Relays[c.GameID][lobby.Name].CloseDone
			log.Printf("Game ID %s lobby %s relay has been destroyed", c.GameID, lobby.Name)
			delete(state.Relays[c.GameID], lobby.Name)
		}

		delete(state.Lobbies[c.GameID], lobby.Name)
		log.Printf("Lobby %s has been destroyed", lobby.Name)

		message.Broadcast(state.UninitializedPeers[c.GameID], structs.Packet{Opcode: "LOBBY_CLOSED", Payload: lobby.Name})

		ShowStatus(state, lobby, c)
	} else {
		log.Println("Destroy Lobby had not effect!")
	}
}

func UpdateState(state *structs.Server, lobby *structs.Lobby, c *structs.Client, newstate int8, is_transitional ...bool) {
	log.Printf("%s %d -> %d\n", c.ID, c.State, newstate)

	// Add to new state with lock. Both locks MUST be acquired and released at the same time!
	state.Lock.Lock()
	c.Lock.Lock()
	defer func(c *structs.Client, state *structs.Server) {
		c.Lock.Unlock()
		state.Lock.Unlock()
	}(c, state)
	func(c *structs.Client, state *structs.Server) {

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
			state.UninitializedPeers[c.GameID] = Without(state.UninitializedPeers[c.GameID], c)

		// The client was a host and the server needs to pick a new host
		case 1:
			if lobby != nil {
				if len(lobby.Clients) > 0 {

					// Pick the next host
					newHost := lobby.Clients[0]
					log.Printf("Peer %s was in state %d and will become state 1\n", newHost.ID, newHost.State)
					newHost.State = 1
					lobby.Host = newHost
					lobby.Clients = Without(lobby.Clients, newHost)
					message.Send(newHost, structs.Packet{Opcode: "TRANSITION", Payload: "host"})
					message.Broadcast(lobby.Clients, structs.Packet{Opcode: "NEW_HOST", Payload: structs.NewPeer{
						UserID:    newHost.ID,
						PublicKey: newHost.PublicKey,
						Username:  newHost.Name,
					}})

				} else {
					log.Printf("Lobby %s has no members.\n", lobby.Name)
				}

				if lobby.Host == c && (newstate == -1 || newstate == 0) {
					log.Printf("Lobby %s host has been cleared since %s was the host\n", lobby.Name, c.ID)
					lobby.Host = nil
				}
			}

		// The client was a member and needs to be removed
		case 2:
			if lobby != nil {

				// Remove the client from the lobby Clients
				lobby.Clients = Without(lobby.Clients, c)
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
			state.UninitializedPeers[c.GameID] = Without(state.UninitializedPeers[c.GameID], c)

			// Notify members the client is leaving
			if lobby != nil {

				// Does nothing if there are no peers
				message.Broadcast(Without(And(lobby.Clients, lobby.Host), c), structs.Packet{Opcode: "PEER_LEFT", Payload: c.ID})
			}

		// Client is now uninitialized
		case 0:
			state.UninitializedPeers[c.GameID] = And(state.UninitializedPeers[c.GameID], c)
			c.Lobby = ""
			message.Send(c, structs.Packet{Opcode: "TRANSITION", Payload: ""})

			if c.LastState == 1 {
				DestroyLobby(state, lobby, c)
			}

		// Client needs to become a host
		case 1:

			// Get the old host
			oldHost := lobby.Host

			// Move the old host to the lobby Clients
			if oldHost != nil {
				log.Printf("Peer %s was in state %d and will become state 2\n", oldHost.ID, oldHost.State)
				oldHost.State = 2
				lobby.Clients = And(lobby.Clients, oldHost)
				message.Send(oldHost, structs.Packet{Opcode: "TRANSITION", Payload: "peer"})
			}

			// Set the new host
			lobby.Host = c
			message.Send(c, structs.Packet{Opcode: "TRANSITION", Payload: "host"})

		// Client needs to become a member
		case 2:
			lobby.Clients = And(lobby.Clients, c)
			message.Send(c, structs.Packet{Opcode: "TRANSITION", Payload: "peer"})
		}

		// Perform cleanup duties
		TriggerCleanup(state, lobby, c)
	}(c, state)
}

func TriggerCleanup(state *structs.Server, lobby *structs.Lobby, c *structs.Client) {
	ShowStatus(state, lobby, c)

	if (lobby != nil) &&
		(len(state.UninitializedPeers[c.GameID]) == 0) &&
		(len(state.Lobbies[c.GameID]) == 0) &&
		(len(state.Relays[c.GameID]) == 0) {

		log.Printf("All Game ID %s storage has been destroyed due the host being nil, having no lobbies, relays, or uninitialized peers", c.GameID)
		delete(state.Lobbies, c.GameID)
		delete(state.UninitializedPeers, c.GameID)
		delete(state.Relays, c.GameID)

		ShowStatus(state, lobby, c)
	}
}

func ShowStatus(state *structs.Server, lobby *structs.Lobby, c *structs.Client) {
	log.Printf("Game ID %s has %d lobbies", c.GameID, len(state.Lobbies[c.GameID]))
	log.Printf("Game ID %s has %d uninitialized peers", c.GameID, len(state.UninitializedPeers[c.GameID]))
	log.Printf("Game ID %s has %d relays", c.GameID, len(state.Relays[c.GameID]))
	if lobby != nil {
		log.Printf("Game %s lobby %s has %d clients", c.GameID, lobby.Name, len(state.Lobbies[c.GameID][lobby.Name].Clients))
	}
}

func ValidateToken(token string) bool {
	// TODO
	return token == "let me in"
}

func CloseWithViolationMessage(c *structs.Client, message string) {
	packet := structs.Packet{Opcode: "VIOLATION", Payload: message}
	log.Println(packet)

	c.Conn.WriteJSON(packet)
	c.Conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseGoingAway, message), time.Now().Add(time.Second))
	c.Conn.Close()
}

func CloseWithWarningMessage(c *structs.Client, message string) {
	packet := structs.Packet{Opcode: "WARNING", Payload: message}
	log.Println(packet)
	c.Conn.WriteJSON(packet)
	c.Conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseGoingAway, message), time.Now().Add(time.Second))
	c.Conn.Close()
}

// get returns the client with the given id from the given slice of clients.
// Returns nil if no client with the given id is found.
func Get(peers []*structs.Client, id string) *structs.Client {
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
func Without(peers []*structs.Client, c *structs.Client) []*structs.Client {

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
func And(peers []*structs.Client, c *structs.Client) []*structs.Client {

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
