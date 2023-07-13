package main

import (
	"os"
	"strconv"
	"time"
)

const (
	llmTimeout = 10 * time.Second
)

var (
	llmHost            = os.Getenv("LLM_HOST")
	secret             = os.Getenv("JWT_SECRET")
	maxConcurrent uint = 1
)

func init() {
	if n, err := strconv.Atoi(os.Getenv("LLM_MAX_CONCURRENT")); err == nil {
		maxConcurrent = uint(n)
	}
}
