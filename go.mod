module github.com/bookingcontrol/booker-booking-svc

go 1.23.0

require (
	// TODO: Раскомментировать после создания booker-contracts-go репозитория
	// github.com/bookingcontrol/booker-contracts-go/v1 v1.0.0
	github.com/IBM/sarama v1.43.0
	github.com/google/uuid v1.6.0
	github.com/jackc/pgx/v5 v5.5.3
	github.com/prometheus/client_golang v1.23.2
	github.com/redis/go-redis/v9 v9.5.1
	github.com/rs/zerolog v1.32.0
	go.opentelemetry.io/otel v1.31.0
	go.opentelemetry.io/otel/exporters/jaeger v1.17.0
	go.opentelemetry.io/otel/sdk v1.31.0
	go.opentelemetry.io/otel/trace v1.31.0
	google.golang.org/grpc v1.69.2
	google.golang.org/protobuf v1.36.8
)
