package manager

import (
	"fmt"

	"git.mikedev101.cc/MikeDEV/signaling/pkg/structs"
)

// GetSession retrieves the session associated with the given ID from the server.
// It returns a pointer to the structs.Session if the session exists, or nil if the session does not exist.
// The function is thread-safe.
func GetSession(s *structs.Server, id string) *structs.Session {
	s.Sessions.Mutex.RLock()
	defer s.Sessions.Mutex.RUnlock()
	if _, ok := s.Sessions.Sessions[id]; !ok {
		return nil
	}
	return s.Sessions.Sessions[id]
}

// CreateSession creates a new session for the given client on the server.
// It returns an error if a session for the client already exists. The function is thread-safe.
func CreateSession(s *structs.Server, client *structs.Client) error {
	if DoesPeerExist(s, client.ULID.String()) {
		return fmt.Errorf("session already exists for %s", client.ULID.String())
	}
	s.Sessions.Mutex.Lock()
	defer s.Sessions.Mutex.Unlock()
	s.Sessions.Sessions[client.ULID.String()] = &structs.Session{Client: client, Reset: make(chan bool), Delete: make(chan bool), Done: make(chan bool)}
	return nil
}

// DeleteSession deletes a session associated with the given client from the server.
// It returns an error if the session does not exist. The function is thread-safe.
func DeleteSession(s *structs.Server, client *structs.Client) error {
	if !DoesPeerExist(s, client.ULID.String()) {
		return fmt.Errorf("session does not exist")
	}
	s.Sessions.Mutex.Lock()
	defer s.Sessions.Mutex.Unlock()
	delete(s.Sessions.Sessions, client.ULID.String())
	return nil
}
