package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

var q = newQueue(maxConcurrent)

type promptRequest struct {
	Prompt string `json:"prompt"`
}

func handleGenerate(w http.ResponseWriter, r *http.Request) {
	requestId := r.Header.Get("X-Request-ID")

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

	if !q.wait(id, done) {
		logWriteErr(w, requestId, err, "Can only access LLM once at a time", http.StatusInternalServerError)
		return
	}
	defer q.release(id)

	tokens := make(chan string)
	errCh := make(chan error)

	resp, err := prompt(p)
	if err != nil {
		logWriteErr(w, requestId, err, "Failed to prompt LLM", http.StatusInternalServerError)
		return
	}

	go generate(resp, tokens, r.Context().Done(), errCh)

	for {
		select {
		case token := <-tokens:
			fmt.Fprint(w, token)
			flusher.Flush()
		case err := <-errCh:
			if err != nil {
				logWriteErr(w, requestId, err, err.Error(), http.StatusInternalServerError)
			}
			return
		case <-done:
			fmt.Println("done")
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
				//lint:ignore ST1005 serving error to frontend
				errCh <- fmt.Errorf("Failed to retrieve token from LLM")
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
	data, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", llmHost, bytes.NewBuffer(data))
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
