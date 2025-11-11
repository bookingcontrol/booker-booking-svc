# booker-booking-svc

Микросервис управления бронированиями столов.

## Описание

Booking Service предоставляет gRPC API для:
- Создания бронирований
- Управления статусами бронирований
- Проверки доступности столов
- Управления временными блокировками (holds)

## Запуск

```bash
go run cmd/booking-svc/main.go
```

## Миграции

```bash
# Накатить миграции
psql -U postgres -d booker -f migrations/002_booking_schema.sql
```

## Docker

```bash
docker build -t ghcr.io/bookingcontrol/booker-booking-svc:v1.0.0 .
```

## Зависимости

- `github.com/bookingcontrol/booker-contracts-go/v1` - Protobuf контракты
