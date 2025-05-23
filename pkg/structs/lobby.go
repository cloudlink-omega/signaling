package structs

import (
	"sync"
)

type Lobby struct {
	Name         string
	RelayEnabled bool
	Lock         *sync.RWMutex
	Host         *Client
	Clients      []*Client
	Password     string
	MaxPlayers   int64
	Locked       bool
	GameID       string
	RelayKey     string
}
