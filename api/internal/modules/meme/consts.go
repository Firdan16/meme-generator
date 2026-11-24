package meme

import "time"

const (
	uploadTTL      = 15 * time.Minute
	maxUploadBytes = 10 * 1024 * 1024
	maxImagePixels = 40_000_000
)

const (
	defaultLimit = 20
	maxLimit     = 100
)
