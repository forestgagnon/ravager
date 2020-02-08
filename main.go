package main

import (
	"context"
	"math"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/valyala/fasthttp"

	"github.com/forestgagnon/ravager/internal/config"
)

type Stats struct {
	CompleteCount    uint64
	FailCount        uint64
	TotalCount       uint64
	StatusCounts     []uint64
	LastStatSnapshot atomic.Value // StatSnapshot
}

type StatSnapshot struct {
	Time       time.Time
	TotalCount uint64
}

var client fasthttp.Client
var cfg *config.Config

func init() {
	log.Logger = zerolog.New(os.Stdout).With().Timestamp().Logger()
}

func main() {
	cfg = config.FromFlags()
	doneCtx, done := context.WithCancel(context.Background())

	client = fasthttp.Client{
		MaxConnsPerHost: cfg.Parallelism,
	}

	stats := &Stats{
		// support any three digit status code
		// https://tools.ietf.org/html/rfc7231#section-6
		StatusCounts: make([]uint64, 1000),
	}
	stats.LastStatSnapshot.Store(&StatSnapshot{
		Time:       time.Now(),
		TotalCount: 0,
	})

	go printStatsLoop(doneCtx, stats)

	maxConcurReqSem := make(chan struct{}, cfg.Parallelism)
	wg := sync.WaitGroup{}

	go func() {
		handleReqDone := func() {
			wg.Done()
			<-maxConcurReqSem
		}
		if cfg.NumRequests == 0 {
			log.Warn().Msg("WARNING: RUNNING IN INFINITE MODE!")
		}
		for reqNum := uint64(0); reqNum < cfg.NumRequests || cfg.NumRequests == 0; reqNum++ {
			select {
			case maxConcurReqSem <- struct{}{}:
				wg.Add(1)
				go req(handleReqDone, stats)
			}
		}
		done()
	}()

	quit := make(chan os.Signal)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	select {
	case <-quit:
		printStats(stats)
	case <-doneCtx.Done():
		wg.Wait()
		printStats(stats)
		log.Info().Msg("All done!")
	}
}

func req(done func(), stats *Stats) {
	defer done()
	req := fasthttp.AcquireRequest()
	req.SetRequestURI(cfg.URL)
	req.Header.SetMethod(cfg.Method)
	req.Header.Set("Connection", "close")

	for _, h := range cfg.Headers {
		split := strings.SplitN(h, ":", 2)
		if len(split) != 2 {
			log.Fatal().Msgf(
				"Invalid header, got %d elements when splitting header, needed 2",
				len(split),
			)
		}
		req.Header.Set(split[0], split[1])
	}
	req.SetBody(cfg.Body)

	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseResponse(resp)

	err := client.Do(req, resp)
	fasthttp.ReleaseRequest(req)
	if err != nil {
		atomic.AddUint64(&stats.FailCount, uint64(1))
		log.Error().Err(err).Msg("request failed")
	} else {
		atomic.AddUint64(&stats.CompleteCount, uint64(1))
		// Avoid panics if status code is too high
		if statusCode := resp.StatusCode(); statusCode < len(stats.StatusCounts) {
			atomic.AddUint64(&stats.StatusCounts[resp.StatusCode()], 1)
		}
	}
	atomic.AddUint64(&stats.TotalCount, uint64(1))
}

func printStatsLoop(ctx context.Context, stats *Stats) {
	for {
		printStats(stats)
		select {
		case <-time.After(time.Second):
		case <-ctx.Done():
			return
		}
	}
}

func printStats(stats *Stats) {
	total := atomic.LoadUint64(&stats.TotalCount)

	statuses := map[string]interface{}{
		"statusCodes": presentStatuses(stats.StatusCounts),
	}

	now := time.Now()
	lastSnapshot := stats.LastStatSnapshot.Load().(*StatSnapshot)
	sinceLast := now.Sub(lastSnapshot.Time)
	rps := float64(total-lastSnapshot.TotalCount) / sinceLast.Seconds()

	stats.LastStatSnapshot.Store(&StatSnapshot{
		Time:       now,
		TotalCount: total,
	})

	log.Info().
		Uint64("completed", atomic.LoadUint64(&stats.CompleteCount)).
		Uint64("failed", atomic.LoadUint64(&stats.FailCount)).
		Uint64("total", total).
		Float64("rps", math.Round(rps)).
		Fields(statuses).
		Msg("stats")
}

func presentStatuses(counts []uint64) map[string]interface{} {
	presented := make(map[string]interface{})
	for code, count := range counts {
		countVal := atomic.LoadUint64(&count)
		if countVal > 0 {
			presented[strconv.Itoa(code)] = countVal
		}
	}
	return presented
}
