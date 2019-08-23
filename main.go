package main

import (
	"context"
	"math"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/valyala/fasthttp"

	"github.com/forestgagnon/ravager/internal/config"
)

type Stats struct {
	CompleteCount    uint64
	FailCount        uint64
	TotalCount       uint64
	StatusCounts     map[int]*uint64
	LastStatSnapshot atomic.Value // StatSnapshot
}

type StatSnapshot struct {
	Time       time.Time
	TotalCount uint64
}

var client fasthttp.Client
var cfg *config.Config

func main() {
	cfg = config.FromFlags()
	doneCtx, done := context.WithCancel(context.Background())

	client = fasthttp.Client{
		MaxConnsPerHost: cfg.Parallelism,
		ReadTimeout:     cfg.Timeout,
		WriteTimeout:    cfg.Timeout,
		Dial: func(addr string) (net.Conn, error) {
			return fasthttp.DialTimeout(addr, cfg.Timeout)
		},
	}

	stats := &Stats{
		StatusCounts: make(map[int]*uint64),
	}
	stats.LastStatSnapshot.Store(&StatSnapshot{
		Time:       time.Now(),
		TotalCount: 0,
	})

	// Hydrate http status code map
	for i := 100; i < 600; i++ {
		count := uint64(0)
		stats.StatusCounts[i] = &count
	}

	go printStatsLoop(doneCtx, stats)
	wg := sync.WaitGroup{}
	bringThePain(done, &wg, stats)

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

func bringThePain(done func(), wg *sync.WaitGroup, stats *Stats) {
	go func() {
		if cfg.NumRequests == 0 {
			log.Warn().Msg("WARNING: RUNNING IN INFINITE MODE!")
		}
		if cfg.ParallelismMode == config.ModeMaxInFlight {
			maxConcurReqSem := make(chan struct{}, cfg.Parallelism)
			handleReqDone := func() {
				wg.Done()
				<-maxConcurReqSem
			}
			performRequest := func() {
				req(handleReqDone, stats)
			}
			cappedInFlightDispatch(maxConcurReqSem, wg, performRequest)
		} else if cfg.ParallelismMode == config.ModeRpsStrict {
			handleReqDone := func() {
				wg.Done()
			}
			performRequest := func() {
				req(handleReqDone, stats)
			}
			client.MaxConnsPerHost = math.MaxInt32 // set to unlimited
			strictRpsDispatch(wg, performRequest)
		}
		done()
	}()
}

func cappedInFlightDispatch(sema chan struct{}, wg *sync.WaitGroup, performRequest func()) {
	for reqNum := uint64(0); reqNum < cfg.NumRequests || cfg.NumRequests == 0; reqNum++ {
		select {
		case sema <- struct{}{}:
			wg.Add(1)
			go performRequest()
		}
	}
}

func strictRpsDispatch(wg *sync.WaitGroup, performRequest func()) {
	uint64para := uint64(cfg.Parallelism)
	for reqNum := uint64(0); reqNum < cfg.NumRequests || cfg.NumRequests == 0; reqNum += uint64para {
		time.Sleep(time.Second)
		wg.Add(cfg.Parallelism)
		for r := 0; r < cfg.Parallelism; r++ {
			go performRequest()
		}
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
		atomic.AddUint64(stats.StatusCounts[resp.StatusCode()], 1)
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
	responsesPerSecond := float64(total-lastSnapshot.TotalCount) / sinceLast.Seconds()

	stats.LastStatSnapshot.Store(&StatSnapshot{
		Time:       now,
		TotalCount: total,
	})

	log.Info().
		Uint64("completed", atomic.LoadUint64(&stats.CompleteCount)).
		Uint64("failed", atomic.LoadUint64(&stats.FailCount)).
		Uint64("total", total).
		Float64("responsesPerSecond", math.Round(responsesPerSecond)).
		Fields(statuses).
		Msg("stats")
}

func presentStatuses(counts map[int]*uint64) map[string]interface{} {
	presented := make(map[string]interface{})
	for code, count := range counts {
		if count == nil {
			panic("damn, son")
		}
		countVal := atomic.LoadUint64(count)
		if countVal > 0 {
			presented[strconv.Itoa(code)] = countVal
		}
	}
	return presented
}
