package peer

import (
	"log"

	"git.mikedev101.cc/MikeDEV/signaling/pkg/structs"
	"github.com/goccy/go-json"
)

// Send marshals the given message using go-json and sends it over the given
// webrtc DataChannel. If the channel is nil, it logs the error and returns
// nil.
func Send(r *structs.Relay, channel string, message interface{}) error {
	if channel == "" {
		log.Printf("Got an empty channel when relaying message: %v", message)
		return nil
	}

	// Marshal the message using go-json instead of interface/json
	bytes, err := json.Marshal(message)
	if err != nil {
		return err
	}

	// Send the message
	if dchannel, exists := r.Channels[channel]; !exists {
		log.Printf("Peer %s was not part of channel %s in relay %v while trying to relay message: %s", r.Peer.ULID, channel, r, message)
		return nil
	} else {
		return dchannel.SendText(string(bytes))
	}
}

// Code sends a RelayPacket over the given DataChannel with the given opcode and
// optional payload. If the channel is nil, it logs the error and returns nil.
//
// The payload is marshaled using go-json. If the origin is not nil, it is
// included in the RelayPacket.
func Code(r *structs.Relay, code string, channel string, message interface{}, origin *structs.PeerInfo) error {
	return Send(r, channel, &structs.RelayPacket{Opcode: code, Payload: message, Origin: origin})
}

// Broadcast sends the given message to all the given DataChannels. If the message is a RelayPacket, it will be marshaled using go-json before being sent. If a channel is nil, it is skipped.
func Broadcast(relays []*structs.Relay, channel string, message interface{}) {
	for _, relay := range relays {
		Send(relay, channel, message)
	}
}
