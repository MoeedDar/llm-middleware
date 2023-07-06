package main

import (
	"os"
	"sync"
	"time"

	ws "github.com/gofiber/contrib/websocket"
	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"
)

const timeout = 10 * time.Second

var llmHost = os.Getenv("LLM_HOST")
var connections = sync.Map{}
var nl = newNLock(1)

func stream(c *ws.Conn) {
	msg := struct {
		Token  string `json:"token"`
		Prompt string `json:"prompt"`
	}{}
	err := c.ReadJSON(&msg)
	if err != nil {
		errMsg := "failed to parse message from client"
		log.Err(err).Msg(errMsg)
		c.WriteJSON(map[string]any{"event": "error", "message": errMsg})
		c.Close()
		return
	}

	// No fucking idea why this works
	closeSign := make(chan struct{})
	go func() {
		defer close(closeSign)
		for {
			_, _, err := c.ReadMessage()
			if err != nil {
				return
			}
		}
	}()

	log.Info().Interface("message", msg).Str("ip", c.RemoteAddr().String()).Msg("successfully recieved message from client")

	token := msg.Token
	sub, auth := auth(token)
	if !auth {
		errMsg := "unauthorised access token"
		log.Error().Msg(errMsg)
		c.WriteJSON(map[string]any{"event": "error", "message": errMsg})
		c.Close()
		return
	}
	prompt := msg.Prompt
	if _, ok := connections.Load(sub); ok {
		errMsg := "you can only access the LLM once at a given time"
		log.Error().Msg(errMsg)
		c.WriteJSON(map[string]any{"event": "error", "message": errMsg})
		c.Close()
		return
	}
	connections.Store(sub, struct{}{})
	defer connections.Delete(sub)

	nl.lock()
	defer nl.unlock()

	llmConn, _, err := websocket.DefaultDialer.Dial(llmHost, nil)
	if err != nil {
		errMsg := "failed to establish connection wtih LLM"
		log.Err(err).Msg(errMsg)
		c.WriteJSON(map[string]any{"event": "error", "message": errMsg})
		c.Close()
		return
	}

	defer func() {
		err := llmConn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		if err != nil {
			log.Err(err).Msg("failed to send close message to LLM")
		}
		llmConn.Close()
		c.Close()
	}()

	if err := llmConn.WriteMessage(websocket.TextMessage, []byte(prompt)); err != nil {
		errMsg := "failed to deliver prompt to LLM"
		log.Err(err).Msg(errMsg)
		c.WriteJSON(map[string]any{"event": "error", "message": errMsg})
		return
	}

	wmu := sync.Mutex{}

	msgChan := make(chan map[string]any, 100)
	go func() {
		defer close(msgChan)
		for {
			if c == nil || llmConn == nil {
				return
			}
			msg := map[string]any{}
			err := llmConn.ReadJSON(&msg)
			if err != nil {
				errMsg := "failed to retrieve token from LLM"
				log.Err(err).Msg(errMsg)
				wmu.Lock()
				c.WriteJSON(map[string]any{"event": "error", "message": errMsg})
				wmu.Unlock()
			}
			msgChan <- msg
			if msg["event"] == "stream_end" {
				return
			}
		}
	}()

loop:
	for {
		select {
		case msg, ok := <-msgChan:
			if !ok {
				break
			}
			wmu.Lock()
			err := c.WriteJSON(msg)
			if err != nil {
				errMsg := "failed to forward message to client"
				log.Err(err).Msg(errMsg)
				c.WriteJSON(map[string]any{"event": "error", "message": errMsg})
				break loop
			}
			wmu.Unlock()
			if msg["event"] == "stream_end" {
				break loop
			}
		case <-time.After(timeout):
			err := "timeout reached, no response from LLM"
			log.Error().Msg(err)
			c.WriteJSON(map[string]any{"event": "error", "message": err})
			break loop
		}
	}
}
