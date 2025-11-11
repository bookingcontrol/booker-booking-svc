package main

import (
	"fmt"
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog/log"
)

func startMetricsServer(port int) {
	// Register standard Go and process metrics (promhttp.Handler() already includes them via DefaultGatherer)
	// But we ensure they're available by using DefaultGatherer
	mux := http.NewServeMux()
	// promhttp.Handler() uses prometheus.DefaultGatherer which includes:
	// - Global registry (with our custom metrics from pkg/metrics)
	// - Go collector (go_info, go_memstats, etc.)
	// - Process collector (process_* metrics)
	mux.Handle("/metrics", promhttp.Handler())

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("Metrics server failed")
		}
	}()

	log.Info().Int("port", port).Msg("Metrics server started")
}

