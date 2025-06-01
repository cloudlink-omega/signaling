package structs

import (
	"regexp"
	"sync"

	"github.com/cloudlink-omega/accounts/pkg/authorization"
	backend "github.com/cloudlink-omega/backend/pkg/database"
	"gorm.io/gorm"
)

type Server struct {
	AuthorizedOriginsStorage []*regexp.Regexp
	Mux                      *sync.RWMutex
	TURNOnly                 bool
	Lock                     *sync.RWMutex
	Relays                   map[string]map[string]*Relay
	Lobbies                  map[string]map[string]*Lobby
	GlobalPeerIDs            map[string][]string
	UninitializedPeers       map[string][]*Client
	DB                       *gorm.DB
	Authorization            *authorization.Auth
	GamesDB                  *backend.Database
}
