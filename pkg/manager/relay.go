package manager

import (
	"fmt"

	"github.com/cloudlink-omega/signaling/pkg/structs"
)

// GetRelay gets a relay peer from the server.
// The returned relay is guaranteed to still exist for the duration of the RLock.
func GetRelay(s *structs.Server, peer *structs.Client) *structs.Relay {
	s.RelayLock.RLock()
	defer s.RelayLock.RUnlock()
	return s.Relays[peer]
}

// DeleteRelay gracefully shuts down a relay peer and then deletes it from the server.
// This function is thread-safe and can be called from any goroutine.
func DeleteRelay(s *structs.Server, peer *structs.Client) {
	s.RelayLock.Lock()
	defer s.RelayLock.Unlock()
	func() {

		// Gracefully shutdown the relay
		s.Relays[peer].RequestShutdown <- true
		<-s.Relays[peer].ShutdownComplete

		// Delete the relay
		delete(s.Relays, peer)
	}()
}

// SetRelay sets a relay peer for the given peer on the server.
// The function is thread-safe and can be called from any goroutine.
// The relay is guaranteed to exist for the duration of the Lock.
func SetRelay(s *structs.Server, peer *structs.Client, relay *structs.Relay) {
	s.RelayLock.Lock()
	defer s.RelayLock.Unlock()
	s.Relays[peer] = relay
}

func GetRelayPeers(s *structs.Server, lobbyid string, gameid string) []*structs.Relay {
	if !DoesLobbyExist(s, lobbyid, gameid) {
		return []*structs.Relay{}
	}

	// Get the lobbies
	s.Games.Mutex.RLock()
	defer s.Games.Mutex.RUnlock()
	lobby := get_lobby(s, gameid, lobbyid)

	// Get the relays based on the lobby clients
	var relays []*structs.Relay
	lobby.Mutex.RLock()
	defer lobby.Mutex.RUnlock()
	for _, client := range lobby.Clients {
		relays = append(relays, s.Relays[client])
	}

	// Return a slice of relay pointers
	return relays
}

// VerifyRelayState checks the validity of a relay packet's state.
// It ensures that the recipient field is set and that the recipient
// exists as a peer on the server. It then verifies whether the recipient
// peer can be retrieved using its ULID and is part of the same lobby
// as the relay. Returns an error if any of these validations fail.
func VerifyRelayState(r *structs.Relay, packet *structs.RelayPacket) error {
	if packet.Recipient == "" {
		return fmt.Errorf("recipient is not set")
	}
	if !DoesPeerExist(r.Server, packet.Recipient) {
		return fmt.Errorf("recipient not found")
	}
	peer := GetByULID(r.Server, packet.Recipient)
	if peer == nil {
		return fmt.Errorf("failed to get recipient peer")
	}
	if !IsClientInLobby(r.Server, r.Lobby, r.UGI.String(), peer) {
		return fmt.Errorf("recipient peer is not in the same lobby")
	}
	return nil
}
