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
	jwtSecret                     = os.Getenv("JWT_SECRET")
	maxConcurrent uint            = 1
	repMan        *replicaManager = newReplicaManager()
)

func init() {
	if n, err := strconv.Atoi(os.Getenv("LLM_MAX_CONCURRENT")); err == nil {
		maxConcurrent = uint(n)
	}
	if err := repMan.loadReplicas("replicas.json"); err != nil {
		panic(err)
	}
}
