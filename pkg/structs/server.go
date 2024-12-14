package structs

import (
	"regexp"
	"sync"

	"github.com/go-playground/validator/v10"
)

type Server struct {
	AuthorizedOriginsStorage []*regexp.Regexp
	Mux                      *sync.RWMutex
	Games                    *GameStore
	Sessions                 *SessionStore
	TURNOnly                 bool
	Relays                   map[*Client]*Relay
	RelayLock                *sync.RWMutex
	PacketValidator          *validator.Validate
	WebsocketConnCounter     uint64
}

type Lobby struct {
	Mutex    sync.RWMutex
	Host     *Client
	Settings *LobbySettings
	Clients  []*Client
}

type Game struct {
	Mutex   sync.RWMutex
	Lobbies map[string]*Lobby
	Clients []*Client
}

type Session struct {
	Client *Client
	Reset  chan bool
	Delete chan bool
	Done   chan bool
	Closed bool
}

type SessionStore struct {
	Mutex    sync.RWMutex
	Sessions map[string]*Session
}

type GameStore struct {
	Mutex sync.RWMutex
	Games map[string]*Game
}
