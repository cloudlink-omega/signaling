package handlers

import (
	"encoding/json"
	"strings"

	"github.com/gofiber/fiber/v2/log"

	"github.com/cloudlink-omega/signaling/pkg/constants"
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

	if state.BypassDB {

		// Try to derive username from token
		if !c.TokenWasPresent {
			c.Token = args.Token

			claims := session.GetClaimsFromToken(state, c.Token)
			c.UserID = claims.ULID
			c.Name = claims.Username
		}

		if c.Name == "" {
			c.Name = args.Username
		}

		if c.UserID == "" {
			// Derive a UserID based on the current instance ID but ONLY the first part, not the UGI
			c.UserID = "GUEST_" + c.InstanceID[:strings.Index(c.InstanceID, "_")]
		}
	}

	if !state.BypassDB {
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

				// Verify the status of the session
				if !session.VerifySession(state, claims) {
					session.CloseWithViolationMessage(c, "session expired or revoked")
					return
				}
			}
		}

		// Check if they are a developer of the game
		var found bool
		for _, dev := range c.Game.Developer.DeveloperMembers {
			if dev.UserID == c.UserID {
				found = true
				break
			}
		}

		if found {

			// Warn the member if the game has not yet been approved
			if !c.Game.Developer.State.Read(constants.DEVELOPER_IS_VERIFIED) {
				message.Send(c, structs.Packet{Opcode: "WARNING", Payload: "The developer profile for this game has not yet been approved by an administrator. Please wait for approval."})
			}

			// Warn the member if the game has not yet been approved
			if !c.Game.State.Read(constants.GAME_IS_VERIFIED) {
				message.Send(c, structs.Packet{Opcode: "WARNING", Payload: "This game has not yet been approved by an administrator. Please wait for approval."})
			}

			// Warn the member if the game isn't active
			if !c.Game.State.Read(constants.GAME_IS_ACTIVE) {
				message.Send(c, structs.Packet{Opcode: "WARNING", Payload: "This game is not marked as active. Players will be unable to join."})
			}

		} else {

			// Kick the player from the game if it's not been approved
			if !c.Game.State.Read(constants.GAME_IS_VERIFIED) {
				session.CloseWithViolationMessage(c, "This game has not yet been approved by an administrator.")
				return
			}

			// Kick the player if the game isn't active
			if !c.Game.State.Read(constants.GAME_IS_ACTIVE) {
				session.CloseWithViolationMessage(c, "This game is not active.")
				return
			}
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
