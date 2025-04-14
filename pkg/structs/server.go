package structs

import (
	"regexp"
	"sync"
)

type Server struct {
	AuthorizedOriginsStorage []*regexp.Regexp
	Mux                      *sync.RWMutex
	TURNOnly                 bool
	Lock                     *sync.RWMutex
	Relays                   map[string]map[string]*Relay
	Lobbies                  map[string]map[string]*Lobby
	UninitializedPeers       map[string][]*Client
}
