package structs

import (
	"github.com/oklog/ulid/v2"
	"github.com/pion/webrtc/v4"
)

type Relay struct {
	Server           *Server
	Conn             *webrtc.PeerConnection
	Channels         map[string]*webrtc.DataChannel
	UGI              ulid.ULID
	Peer             *Client
	Lobby            string // lobby id
	Running          bool
	RequestShutdown  chan bool // used to shutdown the relay.
	ShutdownComplete chan bool // used to wait for shutdown to complete.
}
