package signaling

import (
	"time"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"gorm.io/gorm"

	"github.com/cloudlink-omega/accounts/pkg/authorization"
	srv "github.com/cloudlink-omega/signaling/pkg/signaling"
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
func New(

	// Authorized Origins is a list of origins that are allowed to connect to the signaling server.
	Authorized_Origins []string,

	// TURN Only is a boolean that determines if the signaling server should only be used for TURN.
	TURN_Only bool,

	// Passthrough authorization to the signaling server. If not provided, the server will run without authentication.
	Auth *authorization.Auth,

	// Passthrough a database to the signaling server. If not provided, the server will run without authentication.
	DB *gorm.DB,

	// If true, the database will not be automatically migrated, and the caller is responsible for doing so.
	defer_migrate ...bool,

) *SignalingServer {
	var perform_upgrade bool
	if len(defer_migrate) > 0 {
		perform_upgrade = !defer_migrate[0]
	}

	s := srv.Initialize(Authorized_Origins, TURN_Only, Auth, DB, perform_upgrade)
	srv := &SignalingServer{Server: s}

	// Initialize app
	srv.App = fiber.New()

	// Configure rate limits. Default to 15 connection requests per minute with a sliding window.
	srv.App.Use(limiter.New(limiter.Config{
		Max:               15,
		Expiration:        time.Minute,
		LimiterMiddleware: limiter.SlidingWindow{},
		LimitReached: func(c *fiber.Ctx) error {
			return c.SendStatus(fiber.StatusTooManyRequests)
		},
	}))

	// Configure routes
	srv.App.Use("/", s.Upgrader)
	srv.App.Get("/", websocket.New(s.Handler))

	// Initialize middleware
	srv.App.Use(logger.New())
	srv.App.Use(recover.New())

	// Return created instance
	return srv
}
