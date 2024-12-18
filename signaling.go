package signaling

import (
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"

	srv "github.com/cloudlink-omega/signaling/pkg/signaling"
	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
)

type SignalingServer struct {
	App    *fiber.App
	Server *srv.Server
}

// New initializes a new SignalingServer with the given allowed origins and
// TURN only setting. The returned SignalingServer object contains the
// underlying structs.Server and a func that can be used to mount the
// WebSocket handler on a fiber.Router.
func New(Authorized_Origins []string, TURN_Only bool) *SignalingServer {
	s := srv.Initialize(Authorized_Origins, TURN_Only)
	srv := &SignalingServer{Server: s}

	// Initialize app
	srv.App = fiber.New()

	// Configure routes
	srv.App.Use("/", s.Upgrader)
	srv.App.Get("/", websocket.New(s.Handler))

	// Initialize middleware
	srv.App.Use(logger.New())
	srv.App.Use(recover.New())

	// Return created instance
	return srv
}
