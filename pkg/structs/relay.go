package structs

import (
	peer "github.com/muka/peerjs-go"
)

type Relay struct {
	Handler   *peer.Peer
	Id        string
	Close     chan bool
	CloseDone chan bool
}
