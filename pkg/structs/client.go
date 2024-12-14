package structs

import (
	"sync"

	"github.com/gofiber/contrib/websocket"
	"github.com/oklog/ulid/v2"
)

type Client struct {
	Conn           *websocket.Conn
	Session        uint64
	Username       string
	ULID           ulid.ULID
	UGI            ulid.ULID
	Mode           uint   // 0 - none, 1 - host, 2 - peer
	Authorization  any    // session token
	Lobby          string // lobby id
	InLobby        bool
	Mux            *sync.RWMutex  // To prevent concurrent writes to the websocket connection
	Metadata       map[string]any // arbitrary metadata that the client can specify
	PublicKey      string
	TransitionDone chan bool
}

func (c *Client) ClearMode() {
	c.Mode = 0
}

func (c *Client) SetHostMode() {
	c.Mode = 1
}

func (c *Client) SetPeerMode() {
	c.Mode = 2
}

func (c *Client) AmIAHost() bool {
	return c.Mode == 1
}

func (c *Client) AmIPeer() bool {
	return c.Mode == 2
}

func (c *Client) AmINew() bool {
	return c.Mode == 0
}

func (c *Client) StoreAuthorization(token string) {
	c.Authorization = token
}

func (c *Client) AmIAuthorized() bool {
	return c.Authorization != nil
}

func (c *Client) AmIInALobby() bool {
	return c.InLobby
}

func (c *Client) SetLobby(lobby string) {
	c.Lobby = lobby
	c.InLobby = true
}

func (c *Client) ClearLobby() {
	c.Lobby = ""
	c.InLobby = false
}
