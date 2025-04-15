package relay

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/cloudlink-omega/signaling/pkg/structs"
	peer "github.com/muka/peerjs-go"
	"github.com/oklog/ulid/v2"
	"github.com/pion/webrtc/v3"
)

func SpawnRelay(c *structs.Client, state *structs.Server, lobby_name string) (*structs.Relay, error) {
	if state.Relays[c.GameID] != nil {
		return state.Relays[c.GameID][lobby_name], nil
	}

	config := peer.NewOptions()
	config.PingInterval = 500
	config.Debug = 2
	config.Configuration.ICEServers = []webrtc.ICEServer{
		{
			URLs: []string{"stun:vpn.mikedev101.cc:3478", "stun:vpn.mikedev101.cc:5349"},
		},
		{
			URLs:       []string{"turn:vpn.mikedev101.cc:5349", "turn:vpn.mikedev101.cc:3478"},
			Username:   "free",
			Credential: "free",
		},
	}

	relayid := ulid.MustNew(ulid.Timestamp(time.Now()), rand.Reader).String()
	relayPeer, err := peer.NewPeer(relayid, config)
	if err != nil {
		log.Printf("Failed to create relay peer: %s", err)
		state.Lobbies[c.GameID][lobby_name].RelayEnabled = false
		return nil, err
	}

	relayObj := &structs.Relay{
		Handler:   relayPeer,
		Id:        relayid,
		Close:     make(chan bool),
		CloseDone: make(chan bool),
	}

	if state.Relays[c.GameID] == nil {
		log.Printf("Game %s relay storage has been created\n", c.GameID)
		state.Relays[c.GameID] = make(map[string]*structs.Relay)
	}

	if state.Relays[c.GameID][lobby_name] == nil {
		log.Printf("Game %s lobby %s relay storage has been created\n", c.GameID, lobby_name)
		state.Relays[c.GameID][lobby_name] = relayObj
	}

	log.Printf("Created relay peer %s for game %s lobby %s", relayid, c.GameID, lobby_name)

	go HandleRelay(state, relayObj)
	return relayObj, nil
}

func HandleRelay(_ *structs.Server, r *structs.Relay) {
	p := r.Handler
	defer p.Destroy()

	p.On("connection", func(data any) {
		Conn := data.(*peer.DataConnection)

		Conn.On("open", func(data any) {
			log.Printf("Peer %s Connected to relay %s", Conn.GetPeerID(), r.Id)
		})

		Conn.On("data", func(data any) {

			byteStream := data.([]byte)

			// Trim (UTF-16) header???
			byteStream = byteStream[3:]

			log.Printf("Received data from peer %s in relay %s: %s\n", Conn.GetPeerID(), r.Id, byteStream)

			packet := structs.Packet{}
			err := json.Unmarshal(fmt.Appendf(nil, "%s", byteStream), &packet)
			if err != nil {
				log.Println(err)
				return
			}

			// TODO: handle relay messages
			log.Print("Packet: ", packet)
			switch packet.Opcode {
			case "G_MSG":
				break
			case "P_MSG":
				break
			case "G_VAR":
				break
			case "P_VAR":
				break
			case "G_LIST":
				break
			case "P_LIST":
				break
			}
		})

		Conn.On("close", func(data any) {
			log.Printf("Peer %s disconnected from relay %s", Conn.GetPeerID(), r.Id)
		})

		Conn.On("error", func(data any) {
			log.Printf("Peer %s error in relay %s: %v", Conn.GetPeerID(), r.Id, data)
		})
	})

	p.On("error", func(data any) {
		log.Printf("Relay %s peer error: %v", r.Id, data)
	})

	p.On("close", func(data any) {
		log.Printf("Relay peer %s closed", r.Id)
	})

	<-r.Close
	log.Printf("Relay peer %s got close signal", r.Id)
	r.CloseDone <- true
}
