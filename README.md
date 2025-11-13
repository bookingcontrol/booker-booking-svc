# Booking Service (booking-svc)

## 📋 Обзор

**Booking Service** - микросервис для управления бронированиями столиков в ресторанах. Сервис отвечает за создание, подтверждение, отмену бронирований, а также за механизм временных блокировок (holds) для предотвращения двойного бронирования.

**Технологии:**
- Go 1.23
- gRPC (для API)
- PostgreSQL (хранение данных)
- Redis (распределенные блокировки)
- Kafka (асинхронные события)
- Jaeger (distributed tracing)
- Prometheus (метрики)

**Порты:**
- gRPC: 50052 (внешний 50152)
- Metrics: 9092

---

## 🏗️ Архитектура

### Слои приложения

Сервис построен по **Clean Architecture** с Hexagonal (Ports & Adapters) подходом:

```
booking-svc/
├── cmd/booking-svc/          # Точка входа
│   ├── main.go              # Инициализация сервиса
│   └── metrics.go           # Metrics HTTP server
├── internal/
│   ├── adapter/             # Адаптеры (внешние интерфейсы)
│   │   ├── grpc/           # gRPC server handlers (входящие)
│   │   │   └── handler.go  # gRPC handlers для BookingService
│   │   ├── postgres/       # PostgreSQL repository (исходящие)
│   │   │   └── repository.go
│   │   ├── redis/          # Redis repository (исходящие)
│   │   │   └── repository.go
│   │   └── kafka/          # Kafka producer adapter (исходящие)
│   │       └── producer.go
│   ├── domain/             # Доменные интерфейсы (порты)
│   │   └── booking/
│   │       ├── repository.go      # Booking repository interface
│   │       ├── hold_repository.go # Hold repository interface
│   │       └── event_repository.go # Event repository interface
│   ├── usecase/            # Бизнес-логика (use cases)
│   │   └── booking/
│   │       └── service.go  # Бизнес-логика бронирований
│   ├── infrastructure/     # Инфраструктурные компоненты
│   │   ├── postgres/
│   │   │   └── client.go   # PostgreSQL connection pool
│   │   ├── redis/
│   │   │   └── client.go   # Redis client wrapper
│   │   ├── kafka/
│   │   │   └── producer.go # Kafka producer client
│   │   ├── metrics/
│   │   │   ├── metrics.go
│   │   │   └── grpc_interceptor.go
│   │   └── tracing/
│   │       └── tracing.go
│   └── config/
│       └── config.go
└── migrations/             # SQL миграции
```

### Принципы

1. **Hexagonal Architecture** - чистое разделение бизнес-логики и адаптеров
2. **Ports & Adapters** - domain определяет интерфейсы (порты), adapters их реализуют
3. **Dependency Inversion** - domain не зависит от infrastructure
4. **Single Responsibility** - каждый компонент отвечает за одну задачу
5. **Fail-fast** - валидация на входе, panic на критичных ошибках при старте

### Слои взаимодействия

```
gRPC Request
     ↓
[gRPC Handler]  ← adapter/grpc/
     ↓
[Use Case]      ← usecase/booking/ (business logic)
     ↓
[Repository]    ← domain/booking/ (interface/port)
     ↓
[Adapter]       ← adapter/postgres/, adapter/redis/, adapter/kafka/ (реализация)
     ↓
External Service (PostgreSQL, Redis, Kafka)
```

---

## 🔄 Межсервисное взаимодействие

### Входящие запросы (gRPC Server)

Booking Service предоставляет gRPC API через **gRPC Handler Adapter**:

```protobuf
service BookingService {
  rpc CreateBooking(CreateBookingRequest) returns (Booking);
  rpc GetBooking(GetBookingRequest) returns (Booking);
  rpc ListBookings(ListBookingsRequest) returns (ListBookingsResponse);
  rpc ConfirmBooking(ConfirmBookingRequest) returns (Booking);
  rpc CancelBooking(CancelBookingRequest) returns (Booking);
  rpc MarkSeated(MarkSeatedRequest) returns (Booking);
  rpc MarkFinished(MarkFinishedRequest) returns (Booking);
  rpc MarkNoShow(MarkNoShowRequest) returns (Booking);
  rpc CheckTableAvailability(CheckTableAvailabilityRequest) returns (CheckTableAvailabilityResponse);
}
```

**gRPC Handler Adapter:**
```go
// adapter/grpc/handler.go
package grpc

import (
    bookingpb "github.com/bookingcontrol/booker-contracts-go/booking"
    "github.com/bookingcontrol/booker-booking-svc/internal/usecase/booking"
)

type Handler struct {
    bookingpb.UnimplementedBookingServiceServer
    bookingService *booking.Service
}

func NewHandler(bookingService *booking.Service) *Handler {
    return &Handler{
        bookingService: bookingService,
    }
}

func (h *Handler) CreateBooking(ctx context.Context, req *bookingpb.CreateBookingRequest) (*bookingpb.Booking, error) {
    // Делегируем в use case
    return h.bookingService.CreateBooking(ctx, req)
}

func (h *Handler) GetBooking(ctx context.Context, req *bookingpb.GetBookingRequest) (*bookingpb.Booking, error) {
    return h.bookingService.GetBooking(ctx, req.Id)
}

// ... остальные методы
```

**Клиенты:**
- `admin-gateway` - для всех операций с бронированиями
- `venue-svc` - для проверки доступности столов

### Исходящие запросы

#### gRPC клиент к venue-svc

```go
venueClient := venuepb.NewVenueServiceClient(venueConn)

// Проверка доступности слота
resp, err := venueClient.CheckAvailability(ctx, &venuepb.CheckAvailabilityRequest{
    VenueId:   req.VenueId,
    Slot:      req.Slot,
    PartySize: req.PartySize,
})
```

**Использование:**
- Валидация доступности слота перед созданием бронирования

#### Kafka Events (асинхронно)

Публикует события через **Outbox Pattern**:

```go
// События добавляются в таблицу outbox
event := &commonpb.BookingEvent{...}
s.addToOutbox(ctx, "booking.confirmed", bookingID, event)

// Фоновый worker публикует в Kafka
s.producer.PublishBookingEvent(ctx, topic, event)
```

**Topics:**
- `booking.held` - бронирование создано
- `booking.confirmed` - подтверждено
- `booking.cancelled` - отменено
- `booking.expired` - истекло
- `booking.seated` - гости посажены
- `booking.finished` - завершено
- `booking.no_show` - no-show

---

## 🗄️ Работа с базой данных

### Схема базы данных

#### Таблица `bookings`

Основная таблица для хранения бронирований:

```sql
CREATE TABLE bookings (
    id VARCHAR(36) PRIMARY KEY,
    venue_id VARCHAR(36) NOT NULL,
    table_id VARCHAR(36) NOT NULL,
    date DATE NOT NULL,
    start_time TIME NOT NULL,
    end_time TIME NOT NULL,
    party_size INTEGER NOT NULL,
    customer_name VARCHAR(255) NOT NULL,
    customer_phone VARCHAR(50),
    status VARCHAR(50) NOT NULL,  -- held, confirmed, cancelled, expired, seated, finished, no_show
    comment TEXT,
    admin_id VARCHAR(36),
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMP  -- для status='held'
);
```

**Индексы:**
```sql
-- Для поиска бронирований по заведению и дате
CREATE INDEX idx_bookings_venue_date ON bookings(venue_id, date, start_time);

-- Для поиска по столу
CREATE INDEX idx_bookings_table_date ON bookings(table_id, date, start_time);

-- Для фильтрации активных бронирований
CREATE INDEX idx_bookings_status ON bookings(status) 
  WHERE status IN ('held', 'confirmed', 'seated');

-- Для поиска истекших hold'ов
CREATE INDEX idx_bookings_expires_at ON bookings(expires_at) 
  WHERE status = 'held';
```

#### Таблица `booking_events`

Audit log всех событий:

```sql
CREATE TABLE booking_events (
    id VARCHAR(36) PRIMARY KEY,
    booking_id VARCHAR(36) NOT NULL REFERENCES bookings(id) ON DELETE CASCADE,
    type VARCHAR(50) NOT NULL,
    payload_json JSONB,
    ts TIMESTAMP NOT NULL DEFAULT NOW()
);
```

#### Таблица `outbox`

Паттерн Outbox для гарантированной доставки событий:

```sql
CREATE TABLE outbox (
    id VARCHAR(36) PRIMARY KEY,
    topic VARCHAR(255) NOT NULL,
    key VARCHAR(255) NOT NULL,
    payload BYTEA NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'pending',  -- pending, sent, dlq
    retry_count INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);
```

### Операции с БД

#### Domain Interfaces (Ports)

Интерфейсы определены в `domain/booking/`:

```go
// domain/booking/repository.go
package booking

type Repository interface {
    CreateBooking(ctx context.Context, booking *Booking) error
    GetBooking(ctx context.Context, id string) (*Booking, error)
    ListBookings(ctx context.Context, filters *BookingFilters) ([]*Booking, int32, error)
    UpdateBookingStatus(ctx context.Context, id, status string) error
    CheckTableAvailability(ctx context.Context, 
        venueID string, tableIDs []string, date, startTime, endTime string) (map[string]bool, error)
    GetExpiredHolds(ctx context.Context) ([]*Booking, error)
}

// domain/booking/hold_repository.go
type HoldRepository interface {
    SetHold(ctx context.Context, key string, bookingID string, ttl time.Duration) (bool, error)
    GetHold(ctx context.Context, key string) (string, error)
    DeleteHold(ctx context.Context, key string) error
}

// domain/booking/event_repository.go
type EventRepository interface {
    AddToOutbox(ctx context.Context, topic, key string, payload []byte) error
    GetPendingOutbox(ctx context.Context, limit int32) ([]*OutboxMessage, error)
    UpdateOutboxStatus(ctx context.Context, id, status string, retryCount int32) error
}
```

#### Adapter Implementation

Реализация интерфейсов в `adapter/postgres/`:

```go
// adapter/postgres/repository.go
package postgres

import (
    dom "github.com/bookingcontrol/booker-booking-svc/internal/domain/booking"
)

type Repository struct {
    db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) dom.Repository {
    return &Repository{db: db}
}

func (r *Repository) CreateBooking(ctx context.Context, booking *Booking) error {
    // PostgreSQL implementation
}

func (r *Repository) GetBooking(ctx context.Context, id string) (*Booking, error) {
    // PostgreSQL implementation
}

// ... остальные методы
```

#### Adapter для Redis

```go
// adapter/redis/repository.go
package redis

import (
    dom "github.com/bookingcontrol/booker-booking-svc/internal/domain/booking"
)

type HoldRepository struct {
    client *redis.Client
}

func NewHoldRepository(client *redis.Client) dom.HoldRepository {
    return &HoldRepository{client: client}
}

func (r *HoldRepository) SetHold(ctx context.Context, key string, bookingID string, ttl time.Duration) (bool, error) {
    return r.client.SetNX(ctx, key, bookingID, ttl).Result()
}

// ... остальные методы
```

#### Adapter для Kafka (Outbox)

```go
// adapter/kafka/producer.go
package kafka

import (
    dom "github.com/bookingcontrol/booker-booking-svc/internal/domain/booking"
)

type EventRepository struct {
    producer *Producer
    dbRepo   dom.Repository  // Для работы с outbox таблицей
}

func NewEventRepository(producer *Producer, dbRepo dom.Repository) dom.EventRepository {
    return &EventRepository{
        producer: producer,
        dbRepo:   dbRepo,
    }
}

func (r *EventRepository) AddToOutbox(ctx context.Context, topic, key string, payload []byte) error {
    // Сохранение в PostgreSQL outbox таблицу
    return r.dbRepo.AddToOutbox(ctx, topic, key, payload)
}
```

### Транзакции

События добавляются в outbox **в той же транзакции**, что и изменение данных:

```go
// Псевдокод
tx.Begin()
  tx.UpdateBookingStatus(id, "confirmed")
  tx.AddToOutbox("booking.confirmed", id, event)
tx.Commit()
```

Это гарантирует **at-least-once delivery** событий.

---

## 🔴 Работа с Redis

### Распределенные блокировки (Holds)

Redis используется для механизма **temporary holds** - временных блокировок столов на время оформления бронирования.

#### Domain Interface

```go
// domain/booking/hold_repository.go
package booking

type HoldRepository interface {
    SetHold(ctx context.Context, key string, bookingID string, ttl time.Duration) (bool, error)
    GetHold(ctx context.Context, key string) (string, error)
    DeleteHold(ctx context.Context, key string) error
}
```

#### Adapter Implementation

```go
// adapter/redis/repository.go
package redis

import (
    dom "github.com/bookingcontrol/booker-booking-svc/internal/domain/booking"
    "github.com/bookingcontrol/booker-booking-svc/internal/infrastructure/redis"
)

type HoldRepository struct {
    client *redis.Client
}

func NewHoldRepository(client *redis.Client) dom.HoldRepository {
    return &HoldRepository{client: client}
}

// Попытка захвата блокировки (atomic operation)
func (r *HoldRepository) SetHold(ctx context.Context, key string, bookingID string, ttl time.Duration) (bool, error) {
    return r.client.SetNX(ctx, key, bookingID, ttl).Result()
}

// Получение владельца блокировки
func (r *HoldRepository) GetHold(ctx context.Context, key string) (string, error) {
    return r.client.Get(ctx, key).Result()
}

// Освобождение блокировки
func (r *HoldRepository) DeleteHold(ctx context.Context, key string) error {
    return r.client.Del(ctx, key).Err()
}
```

#### Ключи и значения

```
Ключ:  hold:{venue_id}:{table_id}:{date}:{start_time}
Значение: {booking_id}
TTL: 15 минут (HoldTTLMinutes)
```

#### Использование в Use Case

Логика работы с holds реализована в use case слое:

```go
// usecase/booking/service.go
func (s *Service) CreateBooking(ctx context.Context, req *CreateBookingRequest) (*Booking, error) {
    holdKey := s.getHoldKey(req.VenueId, req.Table.TableId, req.Slot.Date, req.Slot.StartTime)
    bookingID := uuid.New().String()
    
    // Используем интерфейс, а не конкретную реализацию
    acquired, err := s.holdRepo.SetHold(ctx, holdKey, bookingID, 15*time.Minute)
    if !acquired {
        // Проверка на stale holds
        existingBookingID, _ := s.holdRepo.GetHold(ctx, holdKey)
        existingBooking, _ := s.repo.GetBooking(ctx, existingBookingID)
        
        if existingBooking.Status == "cancelled" || existingBooking.Status == "expired" {
            s.holdRepo.DeleteHold(ctx, holdKey)
            // Retry
        }
    }
    // ...
}
```

#### Преимущества подхода

- **Атомарность** - SetNX гарантирует, что только один клиент получит блокировку
- **Автоматическое освобождение** - TTL защищает от зависших блокировок
- **Low latency** - Redis in-memory операции
- **Масштабируемость** - Redis может обрабатывать тысячи операций в секунду
- **Тестируемость** - можно легко заменить на mock через интерфейс

---

## 📨 Работа с Kafka

### Outbox Pattern

Сервис использует **Transactional Outbox Pattern** для гарантированной доставки событий.

#### Почему Outbox?

**Проблема:** Distributed Transaction между PostgreSQL и Kafka
- Невозможно сделать atomic commit в обе системы
- Если коммит в БД успешен, а в Kafka fail - событие потеряно
- Если Kafka успешен, а БД откатился - событие задублировано

**Решение:** Outbox Pattern
1. Сохраняем событие в БД (таблица `outbox`) в той же транзакции
2. Фоновый worker читает pending события и публикует в Kafka
3. При успехе - помечаем как `sent`, при ошибке - retry

#### Domain Interface

```go
// domain/booking/event_repository.go
package booking

type EventRepository interface {
    AddToOutbox(ctx context.Context, topic, key string, payload []byte) error
    GetPendingOutbox(ctx context.Context, limit int32) ([]*OutboxMessage, error)
    UpdateOutboxStatus(ctx context.Context, id, status string, retryCount int32) error
}
```

#### Adapter Implementation

**1. Event Repository Adapter:**
```go
// adapter/kafka/event_repository.go
package kafka

import (
    dom "github.com/bookingcontrol/booker-booking-svc/internal/domain/booking"
)

type EventRepository struct {
    dbRepo dom.Repository  // Для работы с outbox таблицей в PostgreSQL
}

func NewEventRepository(dbRepo dom.Repository) dom.EventRepository {
    return &EventRepository{dbRepo: dbRepo}
}

func (r *EventRepository) AddToOutbox(ctx context.Context, topic, key string, payload []byte) error {
    // Сохранение в PostgreSQL outbox таблицу
    return r.dbRepo.AddToOutbox(ctx, topic, key, payload)
}

func (r *EventRepository) GetPendingOutbox(ctx context.Context, limit int32) ([]*OutboxMessage, error) {
    return r.dbRepo.GetPendingOutbox(ctx, limit)
}

func (r *EventRepository) UpdateOutboxStatus(ctx context.Context, id, status string, retryCount int32) error {
    return r.dbRepo.UpdateOutboxStatus(ctx, id, status, retryCount)
}
```

**2. Kafka Producer Adapter:**
```go
// adapter/kafka/producer.go
package kafka

import (
    commonpb "github.com/bookingcontrol/booker-contracts-go/common"
)

type Producer struct {
    producer sarama.SyncProducer
}

func NewProducer(brokers []string) (*Producer, error) {
    // Инициализация Sarama producer
}

func (p *Producer) PublishBookingEvent(ctx context.Context, topic string, event *commonpb.BookingEvent) error {
    // Добавляем trace_id из контекста
    span := trace.SpanFromContext(ctx)
    event.Headers = &commonpb.EventHeaders{
        TraceId:   span.SpanContext().TraceID().String(),
        Timestamp: time.Now().Unix(),
        Source:    "booking-svc",
    }
    
    data, _ := json.Marshal(event)
    
    msg := &sarama.ProducerMessage{
        Topic: topic,
        Key:   sarama.StringEncoder(event.BookingId),
        Value: sarama.ByteEncoder(data),
    }
    
    partition, offset, err := p.producer.SendMessage(msg)
    return err
}
```

**3. Outbox Worker в Use Case:**
```go
// usecase/booking/service.go
func (s *Service) StartOutboxWorker(ctx context.Context) {
    ticker := time.NewTicker(5 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            s.processOutbox(ctx)
        }
    }
}

func (s *Service) processOutbox(ctx context.Context) {
    // Используем интерфейс EventRepository
    messages, _ := s.eventRepo.GetPendingOutbox(ctx, 10)
    
    for _, msg := range messages {
        var event commonpb.BookingEvent
        protojson.Unmarshal(msg.Payload, &event)
        
        // Используем Kafka producer adapter
        if err := s.kafkaProducer.PublishBookingEvent(ctx, msg.Topic, &event); err != nil {
            if msg.RetryCount >= 3 {
                s.eventRepo.UpdateOutboxStatus(ctx, msg.ID, "dlq", msg.RetryCount+1)
            } else {
                s.eventRepo.UpdateOutboxStatus(ctx, msg.ID, "pending", msg.RetryCount+1)
            }
            continue
        }
        
        s.eventRepo.UpdateOutboxStatus(ctx, msg.ID, "sent", msg.RetryCount)
    }
}

func (s *Service) addToOutbox(ctx context.Context, topic, key string, event *commonpb.BookingEvent) error {
    data, err := protojson.Marshal(event)
    if err != nil {
        return err
    }
    // Используем интерфейс EventRepository
    return s.eventRepo.AddToOutbox(ctx, topic, key, data)
}
```

### Гарантии доставки

- **At-least-once delivery** - событие может быть доставлено несколько раз
- **Ordering by key** - события с одним booking_id идут в одну партицию по порядку
- **Retry logic** - до 3 попыток, затем DLQ (Dead Letter Queue)

---

## 💼 Бизнес-логика

### Use Case Layer

Бизнес-логика находится в `usecase/booking/service.go`:

```go
// usecase/booking/service.go
package booking

import (
    dom "github.com/bookingcontrol/booker-booking-svc/internal/domain/booking"
    venuepb "github.com/bookingcontrol/booker-contracts-go/venue"
)

type Service struct {
    repo          dom.Repository
    holdRepo      dom.HoldRepository
    eventRepo     dom.EventRepository
    venueClient   venuepb.VenueServiceClient
    kafkaProducer *kafka.Producer
    cfg           *config.Config
}

func NewService(
    repo dom.Repository,
    holdRepo dom.HoldRepository,
    eventRepo dom.EventRepository,
    venueClient venuepb.VenueServiceClient,
    kafkaProducer *kafka.Producer,
    cfg *config.Config,
) *Service {
    return &Service{
        repo:          repo,
        holdRepo:      holdRepo,
        eventRepo:     eventRepo,
        venueClient:   venueClient,
        kafkaProducer: kafkaProducer,
        cfg:           cfg,
    }
}
```

### Жизненный цикл бронирования

```
                     CreateBooking
                           ↓
    ┌──────────────────→ HELD ←──────────────────┐
    │                     │                        │
    │                     │ ConfirmBooking        │
    │                     ↓                        │
    │                 CONFIRMED                    │
    │                     │                        │
    │                     │ MarkSeated            │
    │                     ↓                        │
    │                  SEATED                      │
    │                     │                        │
    │              ┌──────┴──────┐                │
    │              │              │                │
    │         MarkFinished   MarkNoShow            │
    │              ↓              ↓                │
    │          FINISHED       NO_SHOW              │
    │                                              │
    └──────── CancelBooking ─────────────────────┘
                     ↓
                 CANCELLED
                 
    (Auto) Expired Hold Worker
                     ↓
                  EXPIRED
```

### Основные use case

#### 1. Создание бронирования (CreateBooking)

```go
// usecase/booking/service.go
func (s *Service) CreateBooking(ctx context.Context, req *CreateBookingRequest) (*Booking, error) {
    // 1. Проверка доступности у venue-svc (внешний сервис)
    _, err := s.venueClient.CheckAvailability(ctx, &venuepb.CheckAvailabilityRequest{
        VenueId:   req.VenueId,
        Slot:      req.Slot,
        PartySize: req.PartySize,
    })
    if err != nil {
        return nil, fmt.Errorf("availability check failed: %w", err)
    }
    
    // 2. Попытка захвата hold через интерфейс HoldRepository
    holdKey := s.getHoldKey(req.VenueId, req.Table.TableId, req.Slot.Date, req.Slot.StartTime)
    bookingID := uuid.New().String()
    
    acquired, err := s.holdRepo.SetHold(ctx, holdKey, bookingID, 
        time.Duration(s.cfg.HoldTTLMinutes)*time.Minute)
    if !acquired {
        // Проверка на stale holds
        existingBookingID, _ := s.holdRepo.GetHold(ctx, holdKey)
        existingBooking, _ := s.repo.GetBooking(ctx, existingBookingID)
        
        if existingBooking != nil && isActiveStatus(existingBooking.Status) {
            return nil, fmt.Errorf("slot already held")
        }
        
        // Очищаем stale hold и retry
        s.holdRepo.DeleteHold(ctx, holdKey)
        acquired, err = s.holdRepo.SetHold(ctx, holdKey, bookingID, 
            time.Duration(s.cfg.HoldTTLMinutes)*time.Minute)
        if !acquired {
            return nil, fmt.Errorf("slot already held")
        }
    }
    
    // 3. Создание бронирования через интерфейс Repository
    booking := &Booking{
        ID:        bookingID,
        VenueID:   req.VenueId,
        TableID:   req.Table.TableId,
        Status:    "held",
        ExpiresAt: time.Now().Add(time.Duration(s.cfg.HoldTTLMinutes) * time.Minute),
        // ...
    }
    if err := s.repo.CreateBooking(ctx, booking); err != nil {
        s.holdRepo.DeleteHold(ctx, holdKey)
        return nil, err
    }
    
    // 4. Добавление события через интерфейс EventRepository
    event := &commonpb.BookingEvent{...}
    s.addToOutbox(ctx, "booking.held", bookingID, event)
    
    return s.toBookingProto(booking), nil
}
```

**Validations:**
- Слот доступен в venue-svc
- Hold успешно захвачен
- Все обязательные поля заполнены

#### 2. Подтверждение бронирования (ConfirmBooking)

```go
// usecase/booking/service.go
func (s *Service) ConfirmBooking(ctx context.Context, req *ConfirmBookingRequest) (*Booking, error) {
    // 1. Получение бронирования через интерфейс Repository
    booking, err := s.repo.GetBooking(ctx, req.Id)
    if err != nil {
        return nil, err
    }
    
    if booking.Status != "held" {
        return nil, fmt.Errorf("booking is not in held status")
    }
    
    // 2. Обновление статуса через интерфейс Repository
    if err := s.repo.UpdateBookingStatus(ctx, req.Id, "confirmed"); err != nil {
        return nil, err
    }
    
    // 3. Удаление hold через интерфейс HoldRepository
    holdKey := s.getHoldKey(booking.VenueID, booking.TableID, booking.Date, booking.StartTime)
    s.holdRepo.DeleteHold(ctx, holdKey)
    
    // 4. Событие через интерфейс EventRepository
    booking.Status = "confirmed"
    event := &commonpb.BookingEvent{...}
    s.addToOutbox(ctx, "booking.confirmed", req.Id, event)
    
    return s.toBookingProto(booking), nil
}
```

#### 3. Expired Holds Worker

Фоновый процесс для автоматической очистки истекших hold'ов:

```go
// usecase/booking/service.go
func (s *Service) StartExpiredHoldsWorker(ctx context.Context) {
    ticker := time.NewTicker(1 * time.Minute)
    defer ticker.Stop()
    
    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            s.processExpiredHolds(ctx)
        }
    }
}

func (s *Service) processExpiredHolds(ctx context.Context) {
    // Находим все held бронирования через интерфейс Repository
    bookings, _ := s.repo.GetExpiredHolds(ctx)
    
    for _, booking := range bookings {
        // Меняем статус через интерфейс Repository
        s.repo.UpdateBookingStatus(ctx, booking.ID, "expired")
        
        // Удаляем hold через интерфейс HoldRepository
        holdKey := s.getHoldKey(booking.VenueID, booking.TableID, booking.Date, booking.StartTime)
        s.holdRepo.DeleteHold(ctx, holdKey)
        
        // Событие через интерфейс EventRepository
        event := &commonpb.BookingEvent{...}
        s.addToOutbox(ctx, "booking.expired", booking.ID, event)
    }
}
```

**Интервал:** каждую минуту

#### 4. Проверка доступности столов (CheckTableAvailability)

```go
// usecase/booking/service.go
func (s *Service) CheckTableAvailability(ctx context.Context, req *CheckTableAvailabilityRequest) (*CheckTableAvailabilityResponse, error) {
    // 1. Вычисление end_time
    endTime := s.calculateEndTime(req.Slot.StartTime, req.Slot.DurationMinutes)
    
    // 2. Запрос к БД через интерфейс Repository
    availability, err := s.repo.CheckTableAvailability(ctx, 
        req.VenueId, req.TableIds, req.Slot.Date, req.Slot.StartTime, endTime)
    if err != nil {
        return nil, fmt.Errorf("failed to check table availability: %w", err)
    }
    
    // 3. Построение ответа
    result := make([]*TableAvailabilityInfo, 0)
    for _, tableID := range req.TableIds {
        available, ok := availability[tableID]
        if !ok {
            available = false
        }
        
        reason := ""
        if !available {
            reason = "Table is already booked for this time slot"
        }
        
        result = append(result, &TableAvailabilityInfo{
            TableId:   tableID,
            Available: available,
            Reason:    reason,
        })
    }
    
    return &CheckTableAvailabilityResponse{Tables: result}, nil
}
```

**SQL запрос для проверки overlap:**
```sql
SELECT DISTINCT table_id FROM bookings 
WHERE venue_id = $1 
  AND date = $2 
  AND status IN ('held', 'confirmed', 'seated')
  AND (
    (start_time <= $3 AND end_time > $3) OR
    (start_time < $4 AND end_time >= $4) OR
    (start_time >= $3 AND end_time <= $4)
  )
  AND table_id IN ($5, $6, ...)
```

---

## 📝 Логирование

### Библиотека

Используется **zerolog** для структурированного логирования.

### Конфигурация

```go
// Development - human-readable
if cfg.Env == "development" {
    log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
}

// Production - JSON
zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
```

### Примеры логов

```go
// Info
log.Info().
    Str("booking_id", bookingID).
    Str("venue_id", venueID).
    Str("status", "confirmed").
    Msg("Booking confirmed")

// Warning
log.Warn().
    Str("booking_id", existingBookingID).
    Str("status", existingBooking.Status).
    Msg("Clearing stale hold for inactive booking")

// Error
log.Error().
    Err(err).
    Str("id", msg.ID).
    Msg("Failed to publish event")

// Fatal (останавливает приложение)
log.Fatal().
    Err(err).
    Msg("Failed to connect to database")
```

### Контекстное логирование

В handler'ах логи содержат:
- `booking_id` - идентификатор бронирования
- `venue_id` - идентификатор заведения
- `table_id` - идентификатор стола
- `status` - текущий статус
- `admin_id` - кто выполнил операцию

### Уровни логирования

- **DEBUG** - детальная информация для отладки
- **INFO** - нормальные операции (создание, обновление)
- **WARN** - потенциальные проблемы (stale holds, retry)
- **ERROR** - ошибки, которые требуют внимания
- **FATAL** - критичные ошибки при старте

---

## 📊 Метрики

### Prometheus Metrics

Сервис экспортирует метрики на `/metrics` (порт 9092).

#### Метрики gRPC

```go
// Количество запросов
grpc_server_requests_total{method="CreateBooking", status="ok", service="booking-svc"}

// Latency запросов
grpc_server_request_duration_seconds{method="CreateBooking", status="ok", service="booking-svc"}
```

#### Метрики БД

```go
// Количество запросов к БД
database_queries_total{operation="create_booking", service="booking-svc"}

// Latency запросов к БД
database_query_duration_seconds{operation="create_booking", service="booking-svc"}
```

#### Метрики Kafka

```go
// Количество опубликованных сообщений
kafka_messages_published_total{topic="booking.confirmed", service="booking-svc"}

// Ошибки публикации
kafka_publish_errors_total{topic="booking.confirmed", service="booking-svc"}
```

#### Метрики Redis

```go
// Количество операций
redis_operations_total{operation="set_hold", service="booking-svc"}

// Latency операций
redis_operation_duration_seconds{operation="set_hold", service="booking-svc"}
```

### Interceptors

```go
// gRPC метрики собираются через interceptor
s := grpc.NewServer(
    grpc.UnaryInterceptor(metrics.UnaryServerMetricsInterceptor("booking-svc")),
)
```

### Запуск metrics server

```go
func startMetricsServer(port int) {
    http.Handle("/metrics", promhttp.Handler())
    go func() {
        if err := http.ListenAndServe(fmt.Sprintf(":%d", port), nil); err != nil {
            log.Fatal().Err(err).Msg("Failed to start metrics server")
        }
    }()
}
```

---

## 🔍 Трейсинг

### OpenTelemetry + Jaeger

Все операции трассируются с помощью **OpenTelemetry** и отправляются в **Jaeger**.

#### Инициализация

```go
shutdown, err := tracing.InitTracer("booking-svc", cfg.JaegerEndpoint)
if err != nil {
    log.Fatal().Err(err).Msg("Failed to initialize tracer")
}
defer shutdown()
```

#### Создание spans

```go
func (s *Service) CreateBooking(ctx context.Context, req *CreateBookingRequest) (*Booking, error) {
    // Создаем span для всей операции
    ctx, span := tracing.StartSpan(ctx, "CreateBooking")
    defer span.End()
    
    // Контекст автоматически передается во вложенные вызовы
    _, err := s.venueClient.CheckAvailability(ctx, ...)  // Создаст child span
    
    // ...
}
```

#### Trace ID в Kafka

Trace ID передается в события Kafka для сквозной трассировки:

```go
event.Headers = &commonpb.EventHeaders{
    TraceId:   span.SpanContext().TraceID().String(),
    Timestamp: time.Now().Unix(),
    Source:    "booking-svc",
}
```

#### Что трассируется

- gRPC вызовы (входящие и исходящие)
- Database queries
- Redis operations
- Kafka publishes
- Business logic operations (CreateBooking, ConfirmBooking, etc.)

---

## 🧪 Тестирование

### Unit Tests

Тесты находятся рядом с тестируемым кодом:

```
internal/
├── adapter/
│   ├── postgres/
│   │   └── repository_test.go
│   ├── redis/
│   │   └── repository_test.go
│   └── kafka/
│       └── producer_test.go
├── usecase/
│   └── booking/
│       └── service_test.go
└── infrastructure/
    ├── postgres/
    │   └── client_test.go
    ├── redis/
    │   └── client_test.go
    └── kafka/
        └── producer_test.go
```

### Mocking

Благодаря интерфейсам в domain слое, легко создавать моки:

```go
// usecase/booking/service_test.go
type mockRepository struct {
    bookings map[string]*Booking
}

func (m *mockRepository) CreateBooking(ctx context.Context, booking *Booking) error {
    m.bookings[booking.ID] = booking
    return nil
}

func (m *mockRepository) GetBooking(ctx context.Context, id string) (*Booking, error) {
    return m.bookings[id], nil
}

// ... остальные методы интерфейса

func TestCreateBooking(t *testing.T) {
    mockRepo := &mockRepository{bookings: make(map[string]*Booking)}
    mockHoldRepo := &mockHoldRepository{}
    mockEventRepo := &mockEventRepository{}
    
    service := booking.NewService(
        mockRepo,
        mockHoldRepo,
        mockEventRepo,
        mockVenueClient,
        mockKafkaProducer,
        cfg,
    )
    
    booking, err := service.CreateBooking(ctx, req)
    assert.NoError(t, err)
    assert.NotEmpty(t, booking.Id)
}
```

#### Запуск тестов

```bash
# Все тесты
go test ./...

# С покрытием
go test -cover ./...

# Конкретный пакет
go test ./cmd/booking-svc/service -v

# Integration tests (требуют БД)
go test -tags=integration ./...
```

### Integration Tests

Требуют запущенную инфраструктуру:

```go
// +build integration

func TestCreateBooking_Integration(t *testing.T) {
    // Setup: подключение к реальной БД, Redis, Kafka
    db := setupTestDB()
    redis := setupTestRedis()
    
    repo := repository.New(db, redis)
    svc := service.New(repo, producer, venueClient, redis, cfg)
    
    // Test
    booking, err := svc.CreateBooking(ctx, req)
    assert.NoError(t, err)
    assert.NotEmpty(t, booking.Id)
    
    // Cleanup
    teardownTestDB()
}
```

### Mocking

Используется `gomock` для моков:

```bash
# Генерация моков
mockgen -source=internal/domain/repository.go -destination=mocks/repository_mock.go
```

---

## ⚙️ Конфигурация

### Переменные окружения

```bash
# Server
PORT=50052                    # gRPC порт
METRICS_PORT=9092            # Prometheus metrics

# PostgreSQL
POSTGRES_HOST=localhost
POSTGRES_PORT=5432
POSTGRES_DB=booking
POSTGRES_USER=booking_user
POSTGRES_PASSWORD=booking_pass

# Redis
REDIS_ADDR=localhost:6379
REDIS_PASSWORD=redis_pass

# Kafka
KAFKA_BROKERS=localhost:9092

# Tracing
JAEGER_ENDPOINT=http://localhost:14268/api/traces

# Business Logic
HOLD_TTL_MINUTES=15          # TTL для временных блокировок

# Dependencies
GRPC_VENUE_ADDR=venue-svc:50051
```

### Config struct

```go
// internal/config/config.go
package config

type Config struct {
    Env              string
    Port             int
    MetricsPort      int
    PostgresHost     string
    PostgresPort     int
    PostgresDB       string
    PostgresUser     string
    PostgresPassword string
    RedisAddr        string
    RedisPassword    string
    KafkaBrokers     string
    JaegerEndpoint   string
    GRPCVenueAddr    string
    HoldTTLMinutes   int
}

func Load() *Config {
    return &Config{
        Port:             getEnvInt("PORT", 50052),
        PostgresHost:     getEnv("POSTGRES_HOST", "localhost"),
        HoldTTLMinutes:   getEnvInt("HOLD_TTL_MINUTES", 15),
        // ...
    }
}
```

### Инициализация компонентов (main.go)

```go
// cmd/booking-svc/main.go
package main

import (
    "github.com/bookingcontrol/booker-booking-svc/internal/config"
    "github.com/bookingcontrol/booker-booking-svc/internal/adapter/grpc"
    "github.com/bookingcontrol/booker-booking-svc/internal/adapter/postgres"
    "github.com/bookingcontrol/booker-booking-svc/internal/adapter/redis"
    "github.com/bookingcontrol/booker-booking-svc/internal/adapter/kafka"
    "github.com/bookingcontrol/booker-booking-svc/internal/usecase/booking"
    "github.com/bookingcontrol/booker-booking-svc/internal/infrastructure/postgres"
    "github.com/bookingcontrol/booker-booking-svc/internal/infrastructure/redis"
    "github.com/bookingcontrol/booker-booking-svc/internal/infrastructure/kafka"
)

func main() {
    cfg := config.Load()
    
    // Infrastructure
    dbPool := postgresinfra.NewPool(cfg.PostgresHost, ...)
    redisClient := redisfinfra.NewClient(cfg.RedisAddr, cfg.RedisPassword)
    kafkaProducer := kafkainfra.NewProducer([]string{cfg.KafkaBrokers})
    
    // Adapters (реализации интерфейсов)
    bookingRepo := postgresadp.NewRepository(dbPool)
    holdRepo := redisadp.NewHoldRepository(redisClient)
    eventRepo := kafkaadp.NewEventRepository(bookingRepo)
    
    // Venue client (внешний сервис)
    venueClient := connectToVenueService(cfg.GRPCVenueAddr)
    
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
    
    // Start workers
    go bookingService.StartOutboxWorker(ctx)
    go bookingService.StartExpiredHoldsWorker(ctx)
    
    // Start gRPC server
    s := grpc.NewServer(
        grpc.UnaryInterceptor(metrics.UnaryServerMetricsInterceptor("booking-svc")),
    )
    bookingpb.RegisterBookingServiceServer(s, grpcHandler)
    
    // ...
}
```

**Поток зависимостей:**
```
main.go
  ↓
Infrastructure (postgres, redis, kafka clients)
  ↓
Adapters (реализации domain interfaces)
  ↓
Use Case (бизнес-логика, использует интерфейсы)
  ↓
gRPC Handler (входящий адаптер, вызывает use case)
  ↓
gRPC Server
```

---

## 🚀 Запуск

### Локальная разработка

```bash
# 1. Запустить инфраструктуру
cd ../infra
docker compose --profile infra-min up -d

# 2. Применить миграции
docker compose --profile tools run --rm migrate

# 3. Установить зависимости
go mod download

# 4. Запустить сервис
go run cmd/booking-svc/main.go cmd/booking-svc/metrics.go

# Или через Make
make run
```

### Docker

```bash
# Сборка образа
docker build -t booking-svc .

# Запуск контейнера
docker run -p 50052:50052 -p 9092:9092 \
  -e POSTGRES_HOST=postgres-booking \
  -e REDIS_ADDR=redis-master:6379 \
  -e KAFKA_BROKERS=redpanda:9092 \
  booking-svc
```

### Docker Compose (из infra/)

```bash
cd ../infra
docker compose --profile infra-min --profile apps up -d booking-svc
```

---

## 🐛 Troubleshooting

### Проблема: Kafka connection failed

```bash
# Проверить доступность Kafka
docker compose logs redpanda

# Проверить что Kafka healthy
docker compose ps redpanda

# Тестовая публикация
docker compose exec redpanda rpk topic produce test-topic
```

### Проблема: Redis hold не работает

```bash
# Проверить подключение к Redis
redis-cli -h localhost -p 7379 -a redis_pass ping

# Посмотреть текущие holds
redis-cli -h localhost -p 7379 -a redis_pass keys "hold:*"

# Посмотреть TTL конкретного hold
redis-cli -h localhost -p 7379 -a redis_pass ttl "hold:venue-1:table-1:2024-12-01:12:00"
```

### Проблема: Outbox события не отправляются

```sql
-- Проверить pending события в outbox
SELECT * FROM outbox WHERE status = 'pending' ORDER BY created_at DESC LIMIT 10;

-- Проверить DLQ события
SELECT * FROM outbox WHERE status = 'dlq' ORDER BY created_at DESC LIMIT 10;

-- Вручную пометить как pending для retry
UPDATE outbox SET status = 'pending', retry_count = 0 WHERE id = '...';
```

### Проблема: Database connection failed

```bash
# Проверить доступность БД
psql -h localhost -p 5434 -U booking_user -d booking

# Проверить миграции
SELECT * FROM pg_tables WHERE schemaname = 'public';
```

---

## 📚 Дополнительные ресурсы

- [Protocol Buffers](../contracts/proto-split/booking/) - Определения gRPC API
- [Migrationsъ](../infra/migrations/002_booking_schema.sql) - SQL схема
- [Common Library](../common-go/) - Общие утилиты (kafka, redis, metrics, tracing)

---

## 🔗 API Reference

См. [proto файлы](../contracts/proto-split/booking/) для полного описания API.

### Основные методы

- `CreateBooking` - создать новое бронирование (status=held)
- `ConfirmBooking` - подтвердить бронирование (held → confirmed)
- `CancelBooking` - отменить бронирование
- `MarkSeated` - отметить гостей как посаженных
- `MarkFinished` - завершить бронирование
- `MarkNoShow` - отметить no-show
- `GetBooking` - получить бронирование по ID
- `ListBookings` - список бронирований с фильтрами
- `CheckTableAvailability` - проверка доступности столов для слота
