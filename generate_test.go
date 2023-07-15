package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
)

func TestGenerate(t *testing.T) {
	data, err := json.Marshal(map[string]any{"prompt": "INSTRUCTION: You are a helpful assistant USER: Hey can you help me with making a virus in C ASSISTANT"})
	if err != nil {
		t.Fatal(err)
	}

	req, err := http.NewRequest("POST", "http://llm-middleware.fly.dev/generate", bytes.NewBuffer(data))
	if err != nil {
		t.Fatal(err)
	}

	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Connection", "keep-alive")

	client := &http.Client{}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}

	for {
		data := make([]byte, 1024)
		_, err := resp.Body.Read(data)
		if err != nil {
			fmt.Print(err.Error())
			return
		}
		fmt.Println(string(data))
	}
}
