package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type replica struct {
	host   string
	weight uint
}

func (r replica) ping() bool {
	client := http.Client{
		Timeout: time.Second * 2,
	}
	resp, err := client.Get(r.host)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

type replicaManager struct {
	iter     uint // replica usage counter
	pointer  uint // current replica
	replicas []replica
	mu       sync.Mutex
}

func newReplicaManager() *replicaManager {
	return &replicaManager{
		iter:     0,
		pointer:  0,
		replicas: make([]replica, 0),
		mu:       sync.Mutex{},
	}
}

func (r *replicaManager) addReplica(host string, weight uint) {
	r.replicas = append(r.replicas, replica{host, weight})
}

func (r *replicaManager) loadReplicas(path string) error {
	wd, err := os.Getwd()
	if err != nil {
		return err
	}

	file, err := os.Open(filepath.Join(wd, path))
	if err != nil {
		return err
	}
	defer file.Close()

	var replicaConfig []struct {
		Host   string `json:"host"`
		Weight uint   `json:"weight"`
	}

	err = json.NewDecoder(file).Decode(&replicaConfig)
	if err != nil {
		return err
	}

	for _, config := range replicaConfig {
		r.addReplica(config.Host, config.Weight)
	}
	return nil
}

func (r *replicaManager) newRequest(path string, method string, body any) (*http.Request, error) {
	r.next()
	retries := 0
	for !r.current().ping() {
		r.skip()
		retries++
		if retries > 10 {
			//lint:ignore ST1005 serving error to frontend
			return nil, fmt.Errorf("Failed to access LLM, max retries reached")
		}
	}
	data, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	path, err = url.JoinPath(r.current().host, path)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(method, path, bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}
	return req, nil
}

func (r *replicaManager) current() replica {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.replicas[r.pointer]
}

func (r *replicaManager) next() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.iter++
	if r.iter > r.replicas[r.pointer].weight {
		r.iter = 0
		r.pointer++
	}
	if r.pointer >= uint(len(r.replicas)) {
		r.pointer = 0
	}
}

func (r *replicaManager) skip() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.iter = 0
	r.pointer++
	if r.pointer > uint(len(r.replicas)) {
		r.pointer = 0
	}
}
