package structs

import "github.com/pion/webrtc/v4"

// Declare the packet format for signaling.
type SignalPacket struct {
	Opcode    string    `json:"opcode" validate:"required" label:"opcode"`                               // Required for protocol compliance
	Payload   any       `json:"payload,omitempty" validate:"omitempty,omitnil,required" label:"payload"` // Required for protocol compliance
	Origin    *PeerInfo `json:"origin,omitempty" validate:"omitempty,omitnil" label:"origin"`            // Server -> Client, identifies client that sent the message
	Recipient string    `json:"recipient,omitempty" validate:"omitempty,omitnil" label:"recipient"`      // Client -> Server, identifies client that should receive the message
	Listener  string    `json:"listener,omitempty" validate:"omitempty,omitnil" label:"listener"`        // For clients to listen to server replies
}

type InitPacket struct {
	Opcode   string `json:"opcode" validate:"required" label:"opcode"`
	Payload  any    `json:"payload" validate:"required" label:"payload"` // required,ulid|jwt
	Listener string `json:"listener,omitempty" validate:"omitempty,omitnil" label:"listener"`
}

// JSON structure for signaling INIT_OK response.
type InitOK struct {
	User      string `json:"user"`
	Id        string `json:"id"`
	SessionID any    `json:"session_id"`
	Game      string `json:"game"`
	Developer string `json:"developer"`
}

type HostConfigPacket struct {
	Opcode   string         `json:"opcode" validate:"required" label:"opcode"`
	Payload  *LobbySettings `json:"payload" validate:"required" label:"payload"`
	Listener string         `json:"listener,omitempty" validate:"omitempty,omitnil" label:"listener"` // For clients to listen to server replies
}

type LobbySettings struct {
	LobbyID             string `json:"lobby_id" label:"lobby_id" validate:"required"`
	UseServerRelay      bool   `json:"use_server_relay" validate:"boolean" label:"use_server_relay"`
	AllowHostReclaim    bool   `json:"allow_host_reclaim" validate:"boolean" label:"allow_host_reclaim"`
	AllowPeersToReclaim bool   `json:"allow_peers_to_claim_host" validate:"boolean" label:"allow_peers_to_claim_host"`
	MaximumPeers        int    `json:"max_peers" validate:"min=0" label:"max_peers"`
	Password            string `json:"password" validate:"omitempty,omitnil,max=128" label:"password"`
	Locked              bool   `json:"locked" validate:"boolean" label:"locked"`
	PublicKey           string `json:"pubkey,omitempty" validate:"omitempty,omitnil" label:"pubkey"`
	ReclaimInProgress   bool   `json:"reclaim_in_progress,omitempty" validate:"omitempty,omitnil"` // This is an internal flag, not to be used by clients.
}

// Declare the packet format for the CONFIG_PEER signaling command.
type PeerConfigPacket struct {
	Opcode   string            `json:"opcode" validate:"required" label:"opcode"`
	Payload  *PeerConfigParams `json:"payload" validate:"required_with=LobbyID" label:"payload"`
	Listener string            `json:"listener,omitempty" validate:"omitempty,omitnil" label:"listener"` // For clients to listen to server replies
}

type PeerConfigParams struct {
	LobbyID   string `json:"lobby_id" validate:"required" label:"lobby_id"`
	Password  string `json:"password" validate:"omitempty,max=128" label:"password"`
	PublicKey string `json:"pubkey,omitempty" validate:"omitempty,omitnil" label:"pubkey"`
}

// Declare the packet format for the NEW_HOST signaling event.
type NewHostParams struct {
	ID        string `json:"id"`
	User      string `json:"user"`
	LobbyID   string `json:"lobby_id"`
	PublicKey string `json:"pubkey,omitempty"`
}

// Declare the packet format for the NEW_PEER signaling event.
type NewPeerParams struct {
	ID        string `json:"id"`
	User      string `json:"user"`
	PublicKey string `json:"pubkey,omitempty"`
}

type RootError struct {
	Errors []map[string]string `json:"Validation error"`
}

type PeerInfo struct {
	ID   string `json:"id"`
	User string `json:"user"`
}

type LobbyInfo struct {
	LobbyHostID       string `json:"lobby_host_id"`
	LobbyHostUsername string `json:"lobby_host_username"`
	MaximumPeers      int    `json:"max_peers"`
	CurrentPeers      int    `json:"current_peers"`
	PasswordRequired  bool   `json:"password_required"`
	Reclaimable       bool   `json:"reclaimable"`
}

// Declare the packet format for webrtc relay.
type RelayPacket struct {
	Opcode    string    `json:"opcode" validate:"required" label:"opcode"`                               // Required for protocol compliance
	Payload   any       `json:"payload,omitempty" validate:"omitempty,omitnil,required" label:"payload"` // Required for protocol compliance
	Origin    *PeerInfo `json:"origin,omitempty" validate:"omitempty,omitnil" label:"origin"`            // Relay -> Peer, identifies client that sent the message
	Recipient string    `json:"recipient,omitempty" validate:"omitempty,omitnil" label:"recipient"`      // Peer -> Relay, identifies client that should receive the message
}

// As per CL5 spec, there are two kinds of candidates - data and voice.
var DATA_CANDIDATE uint8 = 0
var VOICE_CANDIDATE uint8 = 1

// Declare the packet format for handling relay candidate data.
type RelayCandidate struct {
	Type     uint8                      `json:"type" validate:"required" label:"type"`
	Contents *webrtc.SessionDescription `json:"contents" validate:"required" label:"contents"`
}
type RelayCandidatePacket struct {
	Opcode    string          `json:"opcode" validate:"required" label:"opcode"`                               // Required for protocol compliance
	Payload   *RelayCandidate `json:"payload,omitempty" validate:"omitempty,omitnil,required" label:"payload"` // Required for protocol compliance
	Origin    *PeerInfo       `json:"origin,omitempty" validate:"omitempty,omitnil" label:"origin"`            // Relay -> Peer, identifies client that sent the message
	Recipient string          `json:"recipient,omitempty" validate:"omitempty,omitnil" label:"recipient"`      // Peer -> Relay, identifies client that should receive the message
}

// Declare the packet format for handling relay inbound ICE data.
type RelayInboundIce struct {
	Type     uint8                    `json:"type" validate:"required" label:"type"`
	Contents *webrtc.ICECandidateInit `json:"contents" validate:"required" label:"contents"`
}

type RelayInboundIcePacket struct {
	Opcode    string           `json:"opcode" validate:"required" label:"opcode"`                               // Required for protocol compliance
	Payload   *RelayInboundIce `json:"payload,omitempty" validate:"omitempty,omitnil,required" label:"payload"` // Required for protocol compliance
	Origin    *PeerInfo        `json:"origin,omitempty" validate:"omitempty,omitnil" label:"origin"`            // Relay -> Peer, identifies client that sent the message
	Recipient string           `json:"recipient,omitempty" validate:"omitempty,omitnil" label:"recipient"`      // Peer -> Relay, identifies client that should receive the message
}

// Declare the packet format for handling relay outbound ICE data.
type RelayOutboundIce struct {
	Type     uint8                `json:"type" validate:"required" label:"type"`
	Contents *webrtc.ICECandidate `json:"contents" validate:"required" label:"contents"`
}

type RelayOutboundIcePacket struct {
	Opcode    string            `json:"opcode" validate:"required" label:"opcode"`                               // Required for protocol compliance
	Payload   *RelayOutboundIce `json:"payload,omitempty" validate:"omitempty,omitnil,required" label:"payload"` // Required for protocol compliance
	Origin    *PeerInfo         `json:"origin,omitempty" validate:"omitempty,omitnil" label:"origin"`            // Relay -> Peer, identifies client that sent the message
	Recipient string            `json:"recipient,omitempty" validate:"omitempty,omitnil" label:"recipient"`      // Peer -> Relay, identifies client that should receive the message
}
