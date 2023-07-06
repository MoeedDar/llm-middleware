package main

import (
	"github.com/gofiber/contrib/websocket"
	"github.com/rs/zerolog/log"
)

func logBroadcastError(c *websocket.Conn, errMsg string, err error) {
	log.Err(err).Msg(errMsg)
	c.WriteJSON(map[string]any{"event": "error", "message": errMsg})
	c.Close()
}
