package structs

type Packet struct {
	Opcode  string `json:"opcode"`
	Payload any    `json:"payload,omitempty"`
}

type CreateLobbyArgs struct {
	Name        string `json:"name"`
	MaxPlayers  int64  `json:"max_players"`
	Password    string `json:"password"`
	Locked      bool   `json:"locked"`
	EnableRelay bool   `json:"enable_relay"`
}

type FindLobbyArgs struct {
	Host             NewPeer `json:"host"`
	MaxPlayers       int64   `json:"max_players"`
	CurrentPlayers   uint64  `json:"current_players"`
	CurrentlyLocked  bool    `json:"currently_locked"`
	PasswordRequired bool    `json:"password_required"`
	RelayEnabled     bool    `json:"relay_enabled"`
}

type ManageLobbyArgs struct {
	Method string `json:"method"`
	Args   any    `json:"args"`
}

type JoinLobbyArgs struct {
	Name     string `json:"name"`
	Password string `json:"password"`
}

type InitArgs struct {
	Token     string `json:"token"`
	PublicKey string `json:"pubkey,omitempty"`
}

type InitResponse struct {
	InstanceID string `json:"instance_id"`
	UserID     string `json:"user_id"`
	Username   string `json:"username"`
}

type NewPeer struct {
	InstanceID string `json:"instance_id"`
	UserID     string `json:"user_id"`
	Username   string `json:"username"`
	PublicKey  string `json:"pubkey,omitempty"`
}
