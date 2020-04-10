package config

import (
	"github.com/rs/zerolog/log"
	"github.com/spf13/pflag"
)

type Config struct {
	Parallelism int
	NumRequests uint64
	URL         string
	Method      string
	Headers     []string
	Body        []byte
}

func New() *Config {
	return &Config{
		Headers: []string{},
		Body:    []byte{},
	}
}

func FromFlags() *Config {
	cfg := New()

	pflag.StringVarP(
		&cfg.URL, "url", "u", "",
		"URL to crush (required)",
	)
	pflag.StringVarP(
		&cfg.Method, "method", "m", "GET",
		"HTTP verb",
	)
	pflag.IntVarP(
		&cfg.Parallelism, "parallelism", "p", 100,
		"Limit on the number of concurrent requests",
	)
	pflag.Uint64VarP(
		&cfg.NumRequests, "numrequests", "n", 1000,
		"How many requests to perform. To perform infinite requests, set this to 0",
	)
	pflag.StringArrayVarP(
		&cfg.Headers, "header", "h", []string{},
		"headers in the format Header:Value",
	)

	bodyStr := ""
	pflag.StringVarP(
		&bodyStr, "body", "b", "",
		"HTTP body",
	)
	pflag.Parse()

	cfg.Body = []byte(bodyStr)

	if cfg.URL == "" {
		log.Fatal().Msg("url is required")
	}
	return cfg
}
