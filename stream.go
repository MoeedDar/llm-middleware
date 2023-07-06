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
	if err := c.ReadJSON(&msg); err != nil {
		logBroadcastError(c, "failed to parse message from client", err)
		c.Close()
		return
	}

	log.Info().Interface("message", msg).Str("ip", c.RemoteAddr().String()).Msg("successfully recieved message from client")

	token := msg.Token
	sub, auth := auth(token)
	if !auth {
		logBroadcastError(c, "unauthorised access token", nil)
		c.Close()
		return
	}
	prompt := msg.Prompt
	if _, ok := connections.Load(sub); ok {
		logBroadcastError(c, "you can only access the LLM once at a given time", nil)
		return
	}

	nl.lock()
	defer nl.unlock()

	connections.Store(sub, struct{}{})
	defer connections.Delete(sub)

	llmConn, _, err := websocket.DefaultDialer.Dial(llmHost, nil)
	if err != nil {
		logBroadcastError(c, "failed to establish connection wtih LLM", err)
		return
	}

	defer func() {
		log.Info().Str("ip", c.RemoteAddr().String()).Msg("successful clean up")
		if err := llmConn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "")); err != nil {
			log.Err(err).Msg("failed to send close message to LLM")
		}
		llmConn.Close()
		c.Close()
	}()

	if err := llmConn.WriteMessage(websocket.TextMessage, []byte(prompt)); err != nil {
		logBroadcastError(c, "failed to deliver prompt to LLM", err)
		return
	}

	wmu := sync.Mutex{}

	msgChan := make(chan map[string]interface{}, 100)
	quit := make(chan struct{})

	defer func() {
		close(quit)
	}()

	go func() {
		for {
			select {
			case <-quit:
				return
			default:
				msg := map[string]interface{}{}
				if err := llmConn.ReadJSON(&msg); err != nil {
					wmu.Lock()
					logBroadcastError(c, "failed to retrieve token from LLM", err)
					wmu.Unlock()
				}
				msgChan <- msg
				if msg["event"] == "stream_end" {
					return
				}
			}
		}
	}()

	go func() {
		for {
			select {
			case <-quit:
				return
			default:
				_, _, err := c.ReadMessage()
				if err != nil {
					msgChan <- map[string]interface{}{"event": "error", "message": err.Error()}
					return
				}
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
				logBroadcastError(c, "failed to forward message to client", err)
				break loop
			}
			wmu.Unlock()
			if msg["event"] == "stream_end" {
				break loop
			}
		case <-time.After(timeout):
			logBroadcastError(c, "timeout reached, no response from LLM", nil)
			break loop
		}
	}
}
