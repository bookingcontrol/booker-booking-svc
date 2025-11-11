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

	commonpb "github.com/bookingcontrol/booker-contracts-go/v1/common"
)

type Producer struct {
	producer sarama.SyncProducer
}

func NewProducer(brokers []string) (*Producer, error) {
	config := sarama.NewConfig()
	config.Producer.Return.Successes = true
	config.Producer.RequiredAcks = sarama.WaitForAll
	config.Producer.Retry.Max = 5

	producer, err := sarama.NewSyncProducer(brokers, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create producer: %w", err)
	}

	return &Producer{producer: producer}, nil
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

func (p *Producer) PublishVenueEvent(ctx context.Context, topic string, event *commonpb.VenueEvent) error {
	span := trace.SpanFromContext(ctx)
	traceID := span.SpanContext().TraceID().String()

	if event.Headers == nil {
		event.Headers = &commonpb.EventHeaders{}
	}
	event.Headers.TraceId = traceID
	event.Headers.Timestamp = getCurrentTimestamp()
	event.Headers.Source = "venue-svc"

	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	key := event.VenueId
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
		Str("venue_id", event.VenueId).
		Msg("Published venue event")

	return nil
}

func (p *Producer) Close() error {
	return p.producer.Close()
}

type Consumer struct {
	consumer sarama.ConsumerGroup
	handler  sarama.ConsumerGroupHandler
}

func NewConsumer(brokers []string, groupID string, handler sarama.ConsumerGroupHandler) (*Consumer, error) {
	config := sarama.NewConfig()
	config.Consumer.Group.Rebalance.Strategy = sarama.NewBalanceStrategyRoundRobin()
	config.Consumer.Offsets.Initial = sarama.OffsetOldest

	consumer, err := sarama.NewConsumerGroup(brokers, groupID, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create consumer: %w", err)
	}

	return &Consumer{
		consumer: consumer,
		handler:  handler,
	}, nil
}

func (c *Consumer) Consume(ctx context.Context, topics []string) error {
	return c.consumer.Consume(ctx, topics, c.handler)
}

func (c *Consumer) Close() error {
	return c.consumer.Close()
}

func getCurrentTimestamp() int64 {
	return time.Now().Unix()
}
