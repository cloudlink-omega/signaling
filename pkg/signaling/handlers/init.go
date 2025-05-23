package handlers

import (
	"encoding/json"

	"github.com/gofiber/fiber/v2/log"

	"github.com/cloudlink-omega/signaling/pkg/signaling/message"
	"github.com/cloudlink-omega/signaling/pkg/signaling/session"
	"github.com/cloudlink-omega/signaling/pkg/structs"
)

func Init(state *structs.Server, c *structs.Client, wsMsg structs.Packet) {
	if c.Valid {
		message.Send(c, structs.Packet{Opcode: "WARNING", Payload: "already authorized"})
		return
	}

	// Try to parse the Payload into args
	var args structs.InitArgs
	rawMessage, err := json.Marshal(wsMsg.Payload)
	if err != nil {
		session.CloseWithViolationMessage(c, err.Error())
		return
	}
	if err := json.Unmarshal(rawMessage, &args); err != nil {
		session.CloseWithViolationMessage(c, err.Error())
		return
	}

	if !c.AuthedWithCookie {
		if !c.TokenWasPresent {
			c.Token = args.Token
		}
		if !session.ValidateToken(state, c.Token) {
			session.CloseWithViolationMessage(c, "unauthorized")
			return
		} else {
			claims := session.GetClaimsFromToken(state, c.Token)
			c.Name = claims.Username
			c.UserID = claims.ULID
			c.AuthedWithCookie = true
		}
	}

	c.Valid = true
	c.PublicKey = args.PublicKey

	// Create game storage if it doesn't exist
	if state.Lobbies[c.GameID] == nil {
		log.Debugf("Game ID %s lobby storage has been created", c.GameID)
		state.Lobbies[c.GameID] = make(map[string]*structs.Lobby)
	}

	// Return INIT_OK
	message.Send(c, structs.Packet{Opcode: "INIT_OK", Payload: structs.InitResponse{
		InstanceID: c.InstanceID,
		UserID:     c.UserID,
		Username:   c.Name,
	}})
}
