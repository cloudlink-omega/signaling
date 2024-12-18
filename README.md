# CL5 Signaling & Packet Relay Backend

Provides WebRTC Signaling & a server-side message relay. A core service for providing CL5 connectivity.

# Requirements
* Go 1.23.1 or newer

# Usage
The signaling server is a standard Fiber v2 app, and can be natively mounted.
```go
package main

import (
    "github.com/gofiber/fiber/v2"
    "github.com/cloudlink-omega/signaling"
)

func main() {

    // Initialize a new Fiber app
    app := fiber.New()

    // . . . 

    // Initialize the Signaling server
    signaling_server := signaling.New(
        []string{"*"}, // Provide a list of whitelisted origins to connect. Using * will permit all origins.
        false,         // TURN Only mode. Set to true to force the internal server relay to only use TURN servers.
    )

    // Mount the application
    app.Mount("/signaling", signaling_server.App)

    // . . .

    // Run the app
    app.Listen("localhost:3000")
}
```

