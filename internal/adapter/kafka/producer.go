package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/IBM/sarama"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"go.opentelemetry.io/otel/trace"

	commonpb "github.com/bookingcontrol/booker-contracts-go/common"
	"github.com/bookingcontrol/booker-booking-svc/internal/infrastructure/kafka"
)

type Producer struct {
	producer *kafka.Producer
}

func NewProducer(infraProducer *kafka.Producer) *Producer {
	return &Producer{producer: infraProducer}
}

func (p *Producer) PublishBookingEvent(ctx context.Context, topic string, event *commonpb.BookingEvent) error {
	span := trace.SpanFromContext(ctx)
	traceID := span.SpanContext().TraceID().String()

	if event.Headers == nil {
		event.Headers = &commonpb.EventHeaders{}
	}
	event.Headers.TraceId = traceID
	event.Headers.Timestamp = getCurrentTimestamp()
	event.Headers.Source = "booking-svc"

	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	key := event.BookingId
	if key == "" {
		key = uuid.New().String()
	}

	msg := &sarama.ProducerMessage{
		Topic: topic,
		Key:   sarama.StringEncoder(key),
		Value: sarama.ByteEncoder(data),
		Headers: []sarama.RecordHeader{
			{Key: []byte("trace_id"), Value: []byte(traceID)},
		},
	}

	partition, offset, err := p.producer.SendMessage(msg)
	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	log.Info().
		Str("topic", topic).
		Int32("partition", partition).
		Int64("offset", offset).
		Str("booking_id", event.BookingId).
		Msg("Published booking event")

	return nil
}

func getCurrentTimestamp() int64 {
	return time.Now().Unix()
}

