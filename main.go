package main

import (
	"net/http"

	"github.com/rs/zerolog/log"
)

func main() {
	http.HandleFunc("/generate", authMiddleware(handleGenerate))
	log.Info().Msg("Listening on http://localhost:8080/")
	log.Fatal().Err(http.ListenAndServe(":8080", nil))
}
