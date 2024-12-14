package manager

import (
	"fmt"
	"slices"

	"git.mikedev101.cc/MikeDEV/signaling/pkg/structs"
)

// WithoutPeer returns a slice of all elements in the given slice of clients that are
// not equal to the given client. Nil elements are also ignored. The returned
// slice is a new slice and does not modify the original slice in any way.
func WithoutPeer(clients []*structs.Client, client *structs.Client) []*structs.Client {
	var b []*structs.Client
	for _, x := range clients {
		if x != nil && x != client {
			b = append(b, x)
		}
	}
	return b
}

// WithoutRelay returns a slice of all elements in the given slice of relays that are
// not equal to the given relay. Nil elements are also ignored. The returned
// slice is a new slice and does not modify the original slice in any way.
func WithoutRelay(relays []*structs.Relay, relay *structs.Relay) []*structs.Relay {
	var b []*structs.Relay
	for _, x := range relays {
		if x != nil && x != relay {
			b = append(b, x)
		}
	}
	return b
}

// GetByULID returns the client associated with the given ULID, or an error if
// no such client exists.
func GetByULID(s *structs.Server, id string) *structs.Client {
	session := GetSession(s, id)
	if session == nil || session.Client == nil {
		return nil
	}
	return session.Client
}

// DoesPeerExist checks if a peer session with the given ID exists on the server.
// It returns true if the session exists, otherwise false.
func DoesPeerExist(s *structs.Server, id string) bool {
	s.Sessions.Mutex.RLock()
	defer s.Sessions.Mutex.RUnlock()
	_, exists := s.Sessions.Sessions[id]
	return exists
}

// DoesGameExist checks if a game with the given ID exists on the server. It returns true if the game exists, otherwise false.
func DoesGameExist(s *structs.Server, gameid string) bool {
	s.Games.Mutex.RLock()
	defer s.Games.Mutex.RUnlock()
	return s.Games.Games[gameid] != nil && s.Games.Games[gameid].Lobbies != nil
}

// AddClientToGame adds the given client to the game with the given ID. If the
// game does not exist, it is created. The client is appended to the game's
// client slice. The function is thread-safe.
func AddClientToGame(s *structs.Server, gameid string, client *structs.Client) {

	// Get the game. Create it if it doesn't exist
	s.Games.Mutex.Lock()
	defer s.Games.Mutex.Unlock()
	game := get_game(s, gameid)

	// Add the client
	game.Mutex.Lock()
	defer game.Mutex.Unlock()
	game.Clients = append(game.Clients, client)
}

// RemoveClientFromGame removes a client from the specified game on the server.
// If the game does not exist, the function will panic. The function locks
// the game's client slice for thread safety while ensuring the client is
// present before attempting removal.
func RemoveClientFromGame(s *structs.Server, gameid string, client *structs.Client) {
	if !DoesGameExist(s, gameid) {
		return
	}
	s.Games.Mutex.Lock()
	defer s.Games.Mutex.Unlock()
	game := get_game(s, gameid)
	game.Mutex.Lock()
	defer game.Mutex.Unlock()
	func() {
		if !slices.Contains(game.Clients, client) {
			return
		}
		i := slices.Index(game.Clients, client)
		if i == -1 || i > len(game.Clients)-1 {
			return
		}
		game.Clients = append(game.Clients[:i], game.Clients[i+1:]...)
	}()
}

// IsClientInGame checks if a client is in the specified game on the server.
// It will panic if the game does not exist. The function is thread-safe.
func IsClientInGame(s *structs.Server, gameid string, client *structs.Client) bool {
	if !DoesGameExist(s, gameid) {
		panic(fmt.Errorf("game %s does not exist", gameid))
	}
	s.Games.Mutex.Lock()
	defer s.Games.Mutex.Unlock()
	game := get_game(s, gameid)
	game.Mutex.RLock()
	defer game.Mutex.RUnlock()
	return slices.Contains(game.Clients, client)
}

// GetGamePeers returns a slice of all clients in the specified game. The
// function will panic if the game does not exist. The function is thread-safe.
func GetGamePeers(s *structs.Server, gameid string) []*structs.Client {
	if !DoesGameExist(s, gameid) {
		panic(fmt.Errorf("game %s does not exist", gameid))
	}
	s.Games.Mutex.Lock()
	defer s.Games.Mutex.Unlock()
	game := get_game(s, gameid)
	game.Mutex.RLock()
	defer game.Mutex.RUnlock()
	return game.Clients
}
