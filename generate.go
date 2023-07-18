package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/rs/zerolog/log"
)

var q = newQueue(maxConcurrent)

type promptRequest struct {
	Prompt    string `json:"prompt"`
	MaxTokens string `json:"max_tokens"`
	TopK      string `json:"top_k"`
	TopP      string `json:"top_p"`
}

func handleGenerate(w http.ResponseWriter, r *http.Request) {
	requestId := r.Header.Get("X-Request-ID")

	timer := time.Now()
	log.Info().Str("request_id", requestId).Msg("Initialising generate request")
	defer log.Info().Str("request_id", requestId).TimeDiff("Time elapsed", time.Now(), timer).Msg("Finishing request")

	flusher, ok := w.(http.Flusher)
	if !ok {
		msg := "Streaming not supported by connection"
		//lint:ignore ST1005 serving error to frontend
		logWriteErr(w, requestId, fmt.Errorf(msg), msg, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	p := promptRequest{}
	err := json.NewDecoder(r.Body).Decode(&p)
	if err != nil {
		logWriteErr(w, requestId, err, "Invalid JSON object as input", http.StatusInternalServerError)
		return
	}
	defer r.Body.Close()

	id := r.Context().Value(contextSubKey).(string)
	done := r.Context().Done()
	log.Info().Str("request_id", requestId).Msg("Entering queue")
	queueTimer := time.Now()
	if !q.wait(id, done) {
		logWriteErr(w, requestId, err, "Can only access LLM once at a time", http.StatusInternalServerError)
		return
	}
	defer q.release(id)
	log.Info().Str("request_id", requestId).TimeDiff("Time elapsed", time.Now(), queueTimer).Msg("Exiting queue")

	tokens := make(chan string)
	errCh := make(chan error)

	resp, err := prompt(p)
	if err != nil {
		logWriteErr(w, requestId, err, "Failed to prompt LLM", http.StatusInternalServerError)
		return
	}

	log.Info().Str("request_id", requestId).Msg("Beginning generation")
	go generate(resp, tokens, done, errCh)

	for {
		select {
		case token := <-tokens:
			fmt.Fprint(w, token)
			flusher.Flush()
		case err := <-errCh:
			if err != nil {
				logWriteErr(w, requestId, err, "Failed to retrieve token from LLM", http.StatusInternalServerError)
			}
			return
		case <-done:
			return
		}
	}
}

func generate(resp *http.Response, tokens chan<- string, done <-chan struct{}, errCh chan<- error) {
	defer close(tokens)
	defer close(errCh)

	for {
		select {
		case <-done:
			return
		case <-time.After(llmTimeout):
			errCh <- fmt.Errorf("LLM timed out after %v", llmTimeout)
			return
		default:
			data := make([]byte, 1024)
			_, err := resp.Body.Read(data)
			if err != nil {
				errCh <- err
				return
			}
			select {
			case tokens <- string(data):
				break
			default:
				return
			}
		}
	}
}

func prompt(p promptRequest) (*http.Response, error) {
	req, err := repMan.newRequest("/generate", "POST", p)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Connection", "keep-alive")

	client := &http.Client{}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	return resp, nil
}
