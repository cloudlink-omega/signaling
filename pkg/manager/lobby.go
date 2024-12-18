package manager

import (
	"fmt"
	"slices"

	"github.com/cloudlink-omega/signaling/pkg/structs"
)

// GetAllLobbies retrieves all lobbies for a given game, excluding the default lobby entry.
func GetAllLobbies(s *structs.Server, gameid string) []string {
	if !DoesGameExist(s, gameid) {
		return []string{}
	}
	s.Games.Mutex.RLock()
	defer s.Games.Mutex.RUnlock()
	lobbies := s.Games.Games[gameid].Lobbies
	keys := make([]string, 0, len(lobbies))
	for key := range lobbies {
		keys = append(keys, key)
	}
	temp := keys[:0]
	for _, x := range keys {
		if x != "default" {
			temp = append(temp, x)
		}
	}
	return temp
}

// IsClientInLobby checks if a given client is in a given lobby in a given game on a server.
// It returns true if the client is in the lobby, false otherwise.
func IsClientInLobby(s *structs.Server, lobbyid string, gameid string, client *structs.Client) bool {
	if !DoesGameExist(s, gameid) {
		return false
	}
	s.Games.Mutex.RLock()
	defer s.Games.Mutex.RUnlock()
	lobby := get_lobby(s, gameid, lobbyid)
	lobby.Mutex.RLock()
	defer lobby.Mutex.RUnlock()
	return slices.Contains(lobby.Clients, client)
}

// GetLobbyPeers retrieves all clients in a given lobby in a given game on a server.
// It returns a slice of client pointers if the game and lobby exist, an empty slice otherwise.
func GetLobbyPeers(s *structs.Server, lobbyid string, gameid string) []*structs.Client {
	if !DoesGameExist(s, gameid) {
		return []*structs.Client{}
	}
	s.Games.Mutex.RLock()
	defer s.Games.Mutex.RUnlock()
	lobby := get_lobby(s, gameid, lobbyid)
	lobby.Mutex.RLock()
	defer lobby.Mutex.RUnlock()
	return lobby.Clients
}

// AddClientToLobby adds a client to a lobby in a game on a server, or creates the lobby if it doesn't exist.
// It does nothing if the client is already in the lobby.
func AddClientToLobby(s *structs.Server, lobbyid string, gameid string, client *structs.Client) {
	s.Games.Mutex.Lock()
	defer s.Games.Mutex.Unlock()
	lobby := get_lobby(s, gameid, lobbyid)
	lobby.Mutex.Lock()
	defer lobby.Mutex.Unlock()
	func() {
		if slices.Contains(lobby.Clients, client) {
			return
		}
		lobby.Clients = append(lobby.Clients, client)
	}()
}

// RemoveClientFromLobby removes a client from a lobby in a game on a server, if it exists.
// It does nothing if the client is not in the lobby or if the lobby doesn't exist.
// It also does nothing if the client doesn't exist on the server.
func RemoveClientFromLobby(s *structs.Server, lobbyid string, gameid string, client *structs.Client) {
	if !DoesPeerExist(s, client.ULID.String()) {
		return
	}
	if !DoesLobbyExist(s, lobbyid, gameid) {
		return
	}
	s.Games.Mutex.Lock()
	defer s.Games.Mutex.Unlock()
	lobby := get_lobby(s, gameid, lobbyid)
	lobby.Mutex.Lock()
	defer lobby.Mutex.Unlock()
	func() {
		if !slices.Contains(lobby.Clients, client) {
			return
		}
		i := slices.Index(lobby.Clients, client)
		if i == -1 || i > len(lobby.Clients)-1 {
			return
		}
		lobby.Clients = append(lobby.Clients[:i], lobby.Clients[i+1:]...)
	}()
}

// DestroyLobby destroys a lobby in a game on a server, removing it from the server's Games map.
// It does nothing if the lobby doesn't exist.
// It locks the server's Games map and the specific game's Lobbies map for thread safety.
func DestroyLobby(s *structs.Server, gameid string, lobbyid string) {
	if !DoesLobbyExist(s, lobbyid, gameid) {
		return
	}
	s.Games.Mutex.Lock()
	defer s.Games.Mutex.Unlock()
	func() {
		s.Games.Games[gameid].Mutex.Lock()
		defer s.Games.Games[gameid].Mutex.Unlock()
		func() {
			delete(s.Games.Games[gameid].Lobbies, lobbyid)
		}()
		if len(s.Games.Games[gameid].Lobbies) == 0 {
			delete(s.Games.Games, gameid)
		}
	}()
}

// SetLobbyHost sets the host of a lobby in a game on a server.
// It does nothing if the lobby doesn't exist.
// It locks the server's Games map and the specific game's Lobbies map for thread safety.
func SetLobbyHost(s *structs.Server, lobbyid string, gameid string, client *structs.Client) {
	if !DoesLobbyExist(s, lobbyid, gameid) {
		return
	}
	s.Games.Mutex.Lock()
	defer s.Games.Mutex.Unlock()
	func() {
		lobby := get_lobby(s, gameid, lobbyid)
		lobby.Mutex.Lock()
		defer lobby.Mutex.Unlock()
		func() {
			lobby.Host = client
		}()
	}()
}

// RemoveLobbyHost removes the host of a lobby in a game on a server.
// It does nothing if the lobby doesn't exist.
// It locks the server's Games map and the specific game's Lobbies map for thread safety.
func RemoveLobbyHost(s *structs.Server, lobbyid string, gameid string, client *structs.Client) {
	if !DoesLobbyExist(s, lobbyid, gameid) {
		return
	}
	s.Games.Mutex.Lock()
	defer s.Games.Mutex.Unlock()
	func() {
		lobby := get_lobby(s, gameid, lobbyid)

		lobby.Mutex.Lock()
		defer lobby.Mutex.Unlock()
		lobby.Host = nil
	}()
}

// GetLobbyHost retrieves the host client of a specified lobby in a given game on the server.
// It returns a pointer to the host client if the lobby exists, or an error if the lobby does not exist.
// The function locks the server's Games map and the specific lobby's Mutex for thread safety.
func GetLobbyHost(s *structs.Server, lobbyid string, gameid string) (*structs.Client, error) {
	if !DoesLobbyExist(s, lobbyid, gameid) {
		return nil, fmt.Errorf("lobby %s in %s does not exist", lobbyid, gameid)
	}
	s.Games.Mutex.Lock()
	defer s.Games.Mutex.Unlock()
	lobby := get_lobby(s, gameid, lobbyid)
	lobby.Mutex.Lock()
	defer lobby.Mutex.Unlock()
	return func() (*structs.Client, error) {
		if lobby.Host == nil {
			return nil, fmt.Errorf("lobby %s in %s host is nil", lobbyid, gameid)
		}
		return lobby.Host, nil
	}()
}

// SetLobbySettings sets the settings of a lobby in a game on a server.
// It panics if the lobby doesn't exist.
// It locks the server's Games map and the specific lobby's Mutex for thread safety.
func SetLobbySettings(s *structs.Server, lobbyid string, gameid string, settings *structs.LobbySettings) {
	if !DoesLobbyExist(s, lobbyid, gameid) {
		panic(fmt.Errorf("lobby %s in %s does not exist", lobbyid, gameid))
	}
	s.Games.Mutex.Lock()
	defer s.Games.Mutex.Unlock()
	lobby := get_lobby(s, gameid, lobbyid)
	lobby.Mutex.Lock()
	defer lobby.Mutex.Unlock()
	lobby.Settings = settings
}

// GetLobbySettings retrieves the settings for a specified lobby in a given game on the server.
// It returns a pointer to the LobbySettings if the lobby exists, or nil if the lobby does not exist.
// The function locks the server's Games map and the specific lobby's Mutex for thread safety.
func GetLobbySettings(s *structs.Server, lobbyid string, gameid string) *structs.LobbySettings {
	if !DoesLobbyExist(s, lobbyid, gameid) {
		return nil
	}
	s.Games.Mutex.Lock()
	defer s.Games.Mutex.Unlock()
	lobby := get_lobby(s, gameid, lobbyid)
	lobby.Mutex.RLock()
	defer lobby.Mutex.RUnlock()
	return lobby.Settings
}

// DoesLobbyExist checks if a lobby with the given lobbyid exists in a game with the given gameid on the server.
// It returns true if the lobby exists, otherwise false. The function acquires a read lock on the server's Games map
// for thread safety while performing the existence checks.
func DoesLobbyExist(s *structs.Server, lobbyid string, gameid string) bool {
	s.Games.Mutex.RLock()
	defer s.Games.Mutex.RUnlock()
	return func() bool {
		if _, exists := s.Games.Games[gameid]; !exists {
			return false
		}
		if s.Games.Games[gameid].Lobbies == nil {
			return false
		}
		if _, exists := s.Games.Games[gameid].Lobbies[lobbyid]; !exists {
			return false
		}
		return true
	}()
}
