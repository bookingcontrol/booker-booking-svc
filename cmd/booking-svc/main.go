package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/bookingcontrol/booker-booking-svc/internal/config"
	grpcadp "github.com/bookingcontrol/booker-booking-svc/internal/adapter/grpc"
	postgresadp "github.com/bookingcontrol/booker-booking-svc/internal/adapter/postgres"
	redisadp "github.com/bookingcontrol/booker-booking-svc/internal/adapter/redis"
	kafkaadp "github.com/bookingcontrol/booker-booking-svc/internal/adapter/kafka"
	"github.com/bookingcontrol/booker-booking-svc/internal/usecase/booking"
	postgresinfra "github.com/bookingcontrol/booker-booking-svc/internal/infrastructure/postgres"
	redisfinfra "github.com/bookingcontrol/booker-booking-svc/internal/infrastructure/redis"
	kafkainfra "github.com/bookingcontrol/booker-booking-svc/internal/infrastructure/kafka"
	"github.com/bookingcontrol/booker-booking-svc/internal/infrastructure/metrics"
	"github.com/bookingcontrol/booker-booking-svc/internal/infrastructure/tracing"
	bookingpb "github.com/bookingcontrol/booker-contracts-go/booking"
	venuepb "github.com/bookingcontrol/booker-contracts-go/venue"
)

func main() {
	cfg := config.Load()

	// Logger
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	if cfg.Env == "development" {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	}

	// Tracing
	shutdown, err := tracing.InitTracer("booking-svc", cfg.JaegerEndpoint)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize tracer")
	}
	defer shutdown()

	// PostgreSQL
	dbPool, err := postgresinfra.NewPool(cfg.PostgresHost, cfg.PostgresPort, cfg.PostgresDB, cfg.PostgresUser, cfg.PostgresPassword)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to database")
	}
	defer dbPool.Close()

	// Redis
	redisClient := redisfinfra.NewClient(cfg.RedisAddr, cfg.RedisPassword)

	// Kafka Producer with retry logic
	kafkaBrokers := []string{cfg.KafkaBrokers}
	var infraProducer *kafkainfra.Producer
	maxRetries := 20
	retryDelay := 3 * time.Second
	log.Info().Strs("brokers", kafkaBrokers).Msg("Attempting to connect to Kafka...")
	for i := 0; i < maxRetries; i++ {
		var err error
		infraProducer, err = kafkainfra.NewProducer(kafkaBrokers)
		if err == nil {
			log.Info().Msg("Kafka producer connected successfully")
			break
		}
		if i < maxRetries-1 {
			log.Warn().Err(err).Int("attempt", i+1).Int("max_retries", maxRetries).Dur("retry_delay", retryDelay).Msg("Failed to create Kafka producer, retrying...")
			time.Sleep(retryDelay)
		} else {
			log.Fatal().Err(err).Int("total_attempts", maxRetries).Msg("Failed to create Kafka producer after all retries")
		}
	}
	if infraProducer == nil {
		log.Fatal().Msg("Kafka producer is nil after retry loop")
	}
	defer infraProducer.Close()

	// Venue gRPC client
	venueConn, err := grpc.Dial(
		cfg.GRPCVenueAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to venue service")
	}
	defer venueConn.Close()
	venueClient := venuepb.NewVenueServiceClient(venueConn)

	// Adapters (реализации domain interfaces)
	bookingRepo := postgresadp.NewRepository(dbPool)
	holdRepo := redisadp.NewHoldRepository(redisClient)
	eventRepo := kafkaadp.NewEventRepository(bookingRepo)
	kafkaProducer := kafkaadp.NewProducer(infraProducer)

	// Use Case (бизнес-логика)
	bookingService := booking.NewService(
		bookingRepo,
		holdRepo,
		eventRepo,
		venueClient,
		kafkaProducer,
		cfg,
	)

	// gRPC Handler (входящий адаптер)
	grpcHandler := grpcadp.NewHandler(bookingService)

	// Start metrics server
	startMetricsServer(cfg.MetricsPort)

	// Start outbox worker
	go bookingService.StartOutboxWorker(context.Background())

	// Start expired holds worker
	go bookingService.StartExpiredHoldsWorker(context.Background())

	// gRPC Server
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.Port))
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to listen")
	}

	s := grpc.NewServer(
		grpc.UnaryInterceptor(metrics.UnaryServerMetricsInterceptor("booking-svc")),
	)
	bookingpb.RegisterBookingServiceServer(s, grpcHandler)

	// Graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Info().Int("port", cfg.Port).Msg("Booking service started")
		if err := s.Serve(lis); err != nil {
			log.Fatal().Err(err).Msg("Server failed")
		}
	}()

	<-ctx.Done()
	log.Info().Msg("Shutting down...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	done := make(chan struct{})
	go func() {
		s.GracefulStop()
		close(done)
	}()

	select {
	case <-done:
		log.Info().Msg("Server stopped gracefully")
	case <-shutdownCtx.Done():
		log.Warn().Msg("Shutdown timeout, forcing stop")
		s.Stop()
	}
}
