package manager

import (
	"sync"

	"github.com/cloudlink-omega/signaling/pkg/structs"
)

// get_lobby is a internal helper function that retrieves a lobby from a server.
// If the lobby doesn't exist, it creates it along with the game it belongs to.
// It returns the lobby.
func get_lobby(s *structs.Server, gameid string, lobbyid string) *structs.Lobby {
	if _, exists := s.Games.Games[gameid]; !exists {
		s.Games.Games[gameid] = &structs.Game{Mutex: sync.RWMutex{}, Lobbies: make(map[string]*structs.Lobby), Clients: make([]*structs.Client, 0)}
	}
	if s.Games.Games[gameid].Lobbies == nil {
		s.Games.Games[gameid].Lobbies = make(map[string]*structs.Lobby)
	}
	if _, exists := s.Games.Games[gameid].Lobbies[lobbyid]; !exists {
		s.Games.Games[gameid].Lobbies[lobbyid] = &structs.Lobby{Mutex: sync.RWMutex{}, Host: nil, Settings: &structs.LobbySettings{}, Clients: make([]*structs.Client, 0)}
	}
	return s.Games.Games[gameid].Lobbies[lobbyid]
}

// get_game is an internal helper function that retrieves a game from a server.
// If the game doesn't exist, it creates it. It returns the game.
func get_game(s *structs.Server, gameid string) *structs.Game {
	if _, exists := s.Games.Games[gameid]; !exists {
		s.Games.Games[gameid] = &structs.Game{Mutex: sync.RWMutex{}, Lobbies: make(map[string]*structs.Lobby), Clients: make([]*structs.Client, 0)}
	}
	return s.Games.Games[gameid]
}
