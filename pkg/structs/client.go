package structs

import (
	"sync"

	"github.com/gofiber/contrib/websocket"
)

type Client struct {
	Conn             *websocket.Conn
	Lock             *sync.Mutex
	TransmitLock     *sync.Mutex
	ID               string
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
}
