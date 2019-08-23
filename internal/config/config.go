package config

import (
	"errors"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/spf13/pflag"
)

type ParallelismMode string

const (
	ModeMaxInFlight ParallelismMode = "max-in-flight"
	ModeRpsStrict   ParallelismMode = "as-rps"
)

type Config struct {
	Parallelism     int
	NumRequests     uint64
	URL             string
	Method          string
	Headers         []string
	Body            []byte
	ParallelismMode ParallelismMode
	Timeout         time.Duration
}

func New() *Config {
	return &Config{
		Headers: []string{},
		Body:    []byte{},
	}
}

func FromFlags() *Config {
	cfg := New()

	pModeFlag := ""
	pflag.StringVar(
		&pModeFlag, "parallelism-mode", string(ModeMaxInFlight),
		fmt.Sprintf("mode (%s, %s)", ModeMaxInFlight, ModeRpsStrict),
	)

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

	pflag.DurationVar(&cfg.Timeout, "timeout", 20*time.Second,
		"client timeout for requests (e.g. 5s or 2m)",
	)

	bodyStr := ""
	pflag.StringVarP(
		&bodyStr, "body", "b", "",
		"HTTP body",
	)
	cfg.Body = []byte(bodyStr)

	pflag.Parse()
	if cfg.URL == "" {
		log.Fatal().Msg("url is required")
	}

	pMode, err := toParallelismMode(pModeFlag)
	if err != nil {
		log.Fatal().Err(err).Msg("fatal config error")
	}
	cfg.ParallelismMode = pMode

	return cfg
}

func toParallelismMode(pMode string) (ParallelismMode, error) {
	cast := ParallelismMode(pMode)
	if cast == ModeMaxInFlight || cast == ModeRpsStrict {
		return cast, nil
	}
	return ParallelismMode(""), errors.New("invalid parallelism mode")
}
