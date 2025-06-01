package structs

import (
	"sync"

	"github.com/cloudlink-omega/storage/pkg/types"
	"github.com/gofiber/contrib/websocket"
)

type Client struct {
	Conn             *websocket.Conn
	Lock             *sync.Mutex
	TransmitLock     *sync.Mutex
	UserID           string
	InstanceID       string
	AuthedWithCookie bool
	Token            string
	TokenWasPresent  bool
	Valid            bool
	State            int8 // -1 - destroyed, 0 - uninitialized, 1 - host, 2 - member
	LastState        int8
	Lobby            string
	PublicKey        string
	Name             string
	GameID           string
	Game             *types.DeveloperGame
}
