package session

import (
	"slices"
	"time"

	"github.com/gofiber/fiber/v2/log"

	account_structs "github.com/cloudlink-omega/accounts/pkg/structs"
	"github.com/cloudlink-omega/signaling/pkg/signaling/message"
	"github.com/cloudlink-omega/signaling/pkg/structs"
	"github.com/gofiber/contrib/websocket"
)

func DestroyLobby(state *structs.Server, lobby *structs.Lobby, c *structs.Client) {
	if lobby != nil && c.LastState == 1 && lobby.Host == nil && len(lobby.Clients) == 0 {
		if lobby.RelayEnabled {
			state.Relays[c.GameID][lobby.Name].Close <- true
			<-state.Relays[c.GameID][lobby.Name].CloseDone
			log.Infof("Game ID %s lobby %s relay has been destroyed", c.GameID, lobby.Name)
			delete(state.Relays[c.GameID], lobby.Name)
		}
		delete(state.Lobbies[c.GameID], lobby.Name)
		log.Infof("Lobby %s has been destroyed", lobby.Name)
		message.Broadcast(state.UninitializedPeers[c.GameID], structs.Packet{Opcode: "LOBBY_CLOSED", Payload: lobby.Name})
	}
}

func UpdateState(state *structs.Server, lobby *structs.Lobby, c *structs.Client, newstate int8, is_transitional ...bool) {
	log.Debugf("%s %d -> %d\n", c.InstanceID, c.State, newstate)

	// Add to new state with lock. Both locks MUST be acquired and released at the same time!
	state.Lock.Lock()
	c.Lock.Lock()
	defer func(c *structs.Client, state *structs.Server) {
		c.Lock.Unlock()
		state.Lock.Unlock()
	}(c, state)
	func(c *structs.Client, state *structs.Server) {

		log.Debugf("Peer %s was in state %d and is now in state %d\n", c.InstanceID, c.State, newstate)

		if lobby == nil {
			// Try to find the lobby given the peer's lobby
			if c.Lobby != "" {
				lobby = state.Lobbies[c.GameID][c.Lobby]
			}
		}

		// First, remove from old global state
		switch c.State {

		case -1:
			log.Warnf("WARNING: Peer", c.InstanceID, "last state was -1")

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
					log.Debugf("Peer %s was in state %d and will become state 1\n", newHost.InstanceID, newHost.State)
					newHost.State = 1
					lobby.Host = newHost
					lobby.Clients = Without(lobby.Clients, newHost)
					message.Send(newHost, structs.Packet{Opcode: "TRANSITION", Payload: "host"})
					message.Broadcast(lobby.Clients, structs.Packet{Opcode: "NEW_HOST", Payload: structs.NewPeer{
						UserID:     newHost.UserID,
						InstanceID: newHost.InstanceID,
						PublicKey:  newHost.PublicKey,
						Username:   newHost.Name,
					}})

				} else {
					log.Debugf("Lobby %s has no members.\n", lobby.Name)
				}

				if lobby.Host == c && (newstate == -1 || newstate == 0) {
					log.Debugf("Lobby %s host has been cleared since %s was the host\n", lobby.Name, c.InstanceID)
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
				message.Broadcast(Without(And(lobby.Clients, lobby.Host), c), structs.Packet{Opcode: "PEER_LEFT", Payload: c.InstanceID})

				// Does nothing if the lobby state isn't ready to be deleted
				if c.LastState == 1 {
					DestroyLobby(state, lobby, c)
				}
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
				log.Debugf("Peer %s was in state %d and will become state 2\n", oldHost.InstanceID, oldHost.State)
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
	if len(state.UninitializedPeers[c.GameID]) == 0 &&
		len(state.Lobbies[c.GameID]) == 0 &&
		len(state.Relays[c.GameID]) == 0 {

		delete(state.Lobbies, c.GameID)
		delete(state.UninitializedPeers, c.GameID)
		delete(state.Relays, c.GameID)
		log.Infof("Game ID %s has been destroyed", c.GameID)
	}
}

func ValidateToken(state *structs.Server, token string) bool {
	return state.Authorization.ValidFromToken(token)
}

func GetClaimsFromToken(state *structs.Server, token string) *account_structs.Claims {
	return state.Authorization.GetClaimsFromToken(token)
}

func CloseWithViolationMessage(c *structs.Client, message string) {
	packet := structs.Packet{Opcode: "VIOLATION", Payload: message}
	log.Debug(packet)

	c.Conn.WriteJSON(packet)
	c.Conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseGoingAway, message), time.Now().Add(time.Second))
	c.Conn.Close()
}

func CloseWithWarningMessage(c *structs.Client, message string) {
	packet := structs.Packet{Opcode: "WARNING", Payload: message}
	log.Debug(packet)
	c.Conn.WriteJSON(packet)
	c.Conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseGoingAway, message), time.Now().Add(time.Second))
	c.Conn.Close()
}

// get returns the client with the given id from the given slice of clients.
// Returns nil if no client with the given id is found.
func Get(peers []*structs.Client, id string) *structs.Client {
	for _, peer := range peers {
		if peer.InstanceID == id {
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
