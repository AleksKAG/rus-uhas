РУС-УХАС

**Российский Ультразвуковой Хирургический Аспиратор с Искусственным интеллектом и Системой телеметрии**

Современная система управления ультразвуковым хирургическим деструктором-аспиратором с AI-классификацией тканей в реальном времени, полной телеметрией и адаптивными протоколами для различных типов операций.

## 🚀 Возможности

### Основные функции
- **AI-классификация тканей** в реальном времени (ONNX Runtime)
- **Адаптивные протоколы** для разных типов операций:
  - Нейрохирургия (высокая точность, низкая мощность)
  - Гепатология (быстрая диссекция печени)
  - Хирургия ран (импульсный режим)
- **Safety-first архитектура** с жесткими ограничениями
- **Полная телеметрия** (Prometheus + Grafana + OpenTelemetry)
- **REST API + WebSocket** для UI
- **JWT аутентификация** по RFID badge

### Технические особенности
- **4 рабочие частоты**: 23, 25, 35, 55 кГц
- **Адаптивная мощность**: 10-50 Вт с автоматической подстройкой
- **Real-time мониторинг**: температура, импеданс, мощность
- **Graceful degradation**: автоматический fallback на эвристику при недоступности AI
- **Structured logging** с correlation ID для аудита
- **OpenTelemetry tracing** для диагностики

## 📁 Структура проекта

```
rus-uhas/
├── cmd/
│   └── control-plane/          # Точка входа основного приложения
│       └── main.go
├── internal/
│   ├── ai/                     # AI-модуль (классификация тканей)
│   │   ├── heuristic_classifier.go
│   │   ├── onnx_classifier.go
│   │   ├── fallback_classifier.go
│   │   └── classifier_test.go
│   ├── domain/                 # Доменные модели и бизнес-логика
│   │   ├── protocol.go
│   │   ├── protocols.go
│   │   ├── protocol_manager.go
│   │   └── protocol_manager_test.go
│   ├── hal/                    # Hardware Abstraction Layer
│   │   ├── interfaces.go
│   │   └── mock/
│   │       └── generator.go
│   └── telemetry/              # Метрики, логи, трассировка
│       ├── metrics.go
│       ├── logging.go
│       └── tracing.go
├── api/
│   ├── grpc/proto/             # gRPC proto-файлы
│   │   ├── embedded.proto
│   │   └── ui.proto
│   └── openapi/                # OpenAPI спецификация
│       └── openapi.yaml
├── deployments/
│   └── docker/                 # Docker Compose для мониторинга
│       ├── docker-compose.yml
│       └── prometheus.yml
├── ml/
│   ├── train_model.py          # Обучение AI модели
│   └── requirements.txt
├── models/                     # ONNX модели (не в git)
├── go.mod
├── Makefile
└── README.md
```

## 🛠️ Установка

### Требования

- **Go** 1.21+
- **Python** 3.8+ (для обучения модели)
- **Docker & Docker Compose** (для мониторинга)
- **ONNX Runtime** (опционально, для production)

### Установка зависимостей

```bash
# Go зависимости
go mod tidy

# Python зависимости (для обучения модели)
cd ml
pip install -r requirements.txt
cd ..
```

## 🚀 Быстрый старт

### 1. Запуск в режиме разработки (Mock HAL)

Самый простой способ запустить систему для разработки и тестирования:

```bash
# Через make
make run

# Или напрямую
export USE_MOCK_HAL=true
export LOG_LEVEL=debug
export HTTP_PORT=8080
go run cmd/control-plane/main.go
```

Система запустится с mock генератором, который имитирует реальное оборудование.

### 2. Запуск с ONNX моделью

Для использования AI-классификации тканей:

```bash
# 1. Обучите модель
make train-model

# 2. Запустите приложение
export USE_MOCK_HAL=true
export ONNX_MODEL_PATH=./models/tissue_classifier.onnx
go run cmd/control-plane/main.go
```

### 3. Запуск с полным стеком мониторинга

```bash
# 1. Запустите Prometheus + Grafana + Jaeger
make docker-up

# 2. Запустите приложение с telemetry
export OTEL_EXPORTER_OTLP_ENDPOINT=localhost:4318
go run cmd/control-plane/main.go

# 3. Откройте Grafana
open http://localhost:3000  # admin/admin
```

## 🧪 Тестирование

```bash
# Запустить все тесты
make test

# Запустить тесты с покрытием
make test-coverage

# Запустить конкретный пакет
go test ./internal/ai -v
go test ./internal/domain -v
```

## 📊 API Endpoints

### Публичные endpoints (без аутентификации)

| Метод | Путь | Описание |
|-------|------|----------|
| GET | `/health` | Health check |
| GET | `/ready` | Readiness check |
| POST | `/api/v1/auth/login` | Аутентификация по RFID badge |

### Защищенные endpoints (требуют JWT)

| Метод | Путь | Описание |
|-------|------|----------|
| GET | `/api/v1/protocols` | Список доступных протоколов |
| GET | `/api/v1/protocols/active` | Получить активный протокол |
| PUT | `/api/v1/protocols/active` | Установить активный протокол |
| GET | `/api/v1/state` | Получить текущее состояние системы |
| POST | `/api/v1/operations` | Начать операцию |
| DELETE | `/api/v1/operations/current` | Остановить операцию |
| GET | `/api/v1/operations/history` | История операций |
| GET | `/api/v1/alerts` | Активные алерты |
| POST | `/api/v1/alerts/{id}/acknowledge` | Подтвердить алерт |
| GET | `/api/v1/settings` | Получить настройки |
| PUT | `/api/v1/settings` | Обновить настройки |
| GET | `/api/v1/user` | Информация о текущем пользователе |
| POST | `/api/v1/auth/logout` | Выход из системы |

### WebSocket

| Метод | Путь | Описание |
|-------|------|----------|
| GET | `/ws` | WebSocket для real-time обновлений (10 Hz) |

### Метрики

| Метод | Путь | Описание |
|-------|------|----------|
| GET | `/metrics` | Prometheus метрики |

## 🔧 Конфигурация

Все параметры настраиваются через переменные окружения:

| Переменная | По умолчанию | Описание |
|------------|--------------|----------|
| `HTTP_PORT` | `8080` | Порт HTTP сервера |
| `METRICS_PATH` | `/metrics` | Путь для Prometheus метрик |
| `CONTROL_LOOP_INTERVAL_MS` | `100` | Интервал цикла управления (мс) |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | `""` | Endpoint для OpenTelemetry (например, `localhost:4318`) |
| `SERVICE_NAME` | `rus-uhas-control-plane` | Имя сервиса для tracing |
| `ONNX_MODEL_PATH` | `./models/tissue_classifier.onnx` | Путь к ONNX модели |
| `USE_MOCK_HAL` | `true` | Использовать mock генератор (для разработки) |
| `DEFAULT_PROTOCOL` | `neuro` | Протокол по умолчанию (`neuro`, `hepatic`, `wound`) |
| `LOG_LEVEL` | `info` | Уровень логирования (`debug`, `info`, `warn`, `error`) |

### Пример конфигурации

```bash
export HTTP_PORT=8080
export CONTROL_LOOP_INTERVAL_MS=50
export OTEL_EXPORTER_OTLP_ENDPOINT=localhost:4318
export ONNX_MODEL_PATH=./models/tissue_classifier.onnx
export USE_MOCK_HAL=false
export DEFAULT_PROTOCOL=hepatic
export LOG_LEVEL=debug
```

## 📈 Мониторинг

### Prometheus метрики

Система экспортирует следующие метрики:

**Генератор:**
- `rus_uhas_generator_power_watts` - Мощность генератора (Вт)
- `rus_uhas_generator_frequency_hz` - Частота генератора (Гц)
- `rus_uhas_generator_tip_temp_celsius` - Температура наконечника (°C)
- `rus_uhas_generator_impedance_ohms` - Импеданс ткани (Ом)
- `rus_uhas_generator_aspiration_bar` - Вакуум аспирации (бар)
- `rus_uhas_generator_irrigation_ml_min` - Поток ирригации (мл/мин)
- `rus_uhas_generator_is_firing` - Статус генерации (0/1)

**Операции:**
- `rus_uhas_operation_duration_seconds` - Длительность операции
- `rus_uhas_operation_tissue_removed_ml` - Объем удаленной ткани (мл)
- `rus_uhas_operation_blood_loss_ml` - Кровопотеря (мл)
- `rus_uhas_operation_control_cycles_total` - Количество циклов управления
- `rus_uhas_operation_errors_total` - Количество ошибок по типам

**AI:**
- `rus_uhas_ai_classification_duration_seconds` - Время классификации тканей
- `rus_uhas_ai_classifications_total` - Количество классификаций по типам тканей

**Протоколы:**
- `rus_uhas_protocol_switches_total` - Количество переключений протоколов
- `rus_uhas_protocol_safety_stops_total` - Количество safety остановок

**Система:**
- `rus_uhas_system_control_loop_duration_seconds` - Длительность цикла управления
- `rus_uhas_system_uptime_seconds` - Время работы системы

### Grafana Dashboard

Импортируйте `deployments/grafana/dashboards/rus-uhas.json` в Grafana для визуализации всех метрик.

Дашборд включает:
- Мощность генератора (gauge)
- Температура наконечника (timeseries)
- Статус генератора (stat)
- Длительность цикла управления (timeseries)
- Ошибки по типам (timeseries)
- Классификация тканей (piechart)
- Safety остановки (stat)

## 🏗️ Архитектура

### Safety Layer

Система имеет жесткие ограничения, которые нельзя обойти:

```go
type SafetyLimits struct {
    MaxPowerWatts:      50.0   // Максимальная мощность (Вт)
    MaxFrequencyHz:     60000  // Максимальная частота (Гц)
    MaxTipTempC:        80.0   // Максимальная температура (°C)
    MaxAspirationBar:   0.9    // Максимальный вакуум (бар)
    MaxIrrigationMlMin: 200.0  // Максимальный поток ирригации (мл/мин)
    MaxImpedanceOhms:   200.0  // Максимальный импеданс (Ом)
}
```

### AI Fallback

Если ONNX модель недоступна, система автоматически переключается на эвристический классификатор:

```
┌─────────────────┐
│  ONNX Classifier│ ──(ошибка)──┐
└─────────────────┘             │
                                ▼
                    ┌───────────────────┐
                    │ Fallback Classifier│
                    └───────────────────┘
                                │
                                ▼
                    ┌───────────────────┐
                    │ Heuristic Classifier│
                    └───────────────────┘
```

### Graceful Shutdown

При остановке системы генератор всегда безопасно отключается:

```
1. Получен сигнал SIGINT/SIGTERM
2. Отмена контекста (остановка control loop)
3. Остановка HTTP сервера
4. Ожидание завершения control loop (таймаут 3 сек)
5. Финальная остановка генератора (safety)
6. Завершение работы
```

## 📝 Примеры использования

### Аутентификация

```bash
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"badge_id": "BADGE001", "pin": "1234"}'
```

Ответ:
```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "expires_at": "2025-01-20T15:00:00Z",
  "user": {
    "id": "user_1",
    "name": "Иванов Иван Иванович",
    "role": "surgeon",
    "specialty": "Нейрохирургия"
  }
}
```

### Получение состояния системы

```bash
curl -X GET http://localhost:8080/api/v1/state \
  -H "Authorization: Bearer <token>"
```

Ответ:
```json
{
  "generator": {
    "is_firing": true,
    "power_watts": 15.0,
    "frequency_hz": 25000.0,
    "tip_temp_c": 42.5,
    "impedance_ohms": 65.0,
    "aspiration_bar": 0.3,
    "irrigation_ml": 50.0
  },
  "active_protocol": "Нейрохирургия",
  "current_user": {
    "id": "user_1",
    "name": "Иванов Иван Иванович",
    "role": "surgeon"
  },
  "timestamp": "2025-01-20T14:30:00Z"
}
```

### Переключение протокола

```bash
curl -X PUT http://localhost:8080/api/v1/protocols/active \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"protocol_id": "hepatic"}'
```

### WebSocket подключение

```javascript
const ws = new WebSocket('ws://localhost:8080/ws');
ws.onmessage = (event) => {
  const data = JSON.parse(event.data);
  console.log('Получено обновление:', data);
};
```

## 🚀 Production deployment

### Сборка бинарника

```bash
make build
```

### Создание Docker образа

```dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o control-plane ./cmd/control-plane

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/control-plane .
COPY --from=builder /app/models ./models
CMD ["./control-plane"]
```

### Запуск в production

```bash
# Установите переменные окружения
export USE_MOCK_HAL=false
export ONNX_MODEL_PATH=/app/models/tissue_classifier.onnx
export OTEL_EXPORTER_OTLP_ENDPOINT=jaeger:4318
export LOG_LEVEL=info

# Запустите
./control-plane
```

## 📚 Документация

- [OpenAPI спецификация](api/openapi/openapi.yaml) - полная документация REST API
- [gRPC Proto файлы](api/grpc/proto/) - контракты для embedded и UI
- [Architecture Decision Records](docs/architecture/) - архитектурные решения

## 🔒 Безопасность

### Реализовано
- ✅ Safety limits (нельзя обойти)
- ✅ JWT аутентификация
- ✅ Graceful shutdown
- ✅ Fallback механизмы
- ✅ Structured logging для аудита

### Рекомендуется для production
- ⚠️ TLS для всех HTTP соединений
- ⚠️ Защита от несанкционированного доступа к embedded
- ⚠️ Резервное копирование данных
- ⚠️ Аудит действий пользователей
- ⚠️ Rate limiting для API

## 🤝 Вклад в проект

Мы приветствуем вклад в проект! Пожалуйста:

1. Fork репозитория
2. Создайте ветку для вашей фичи (`git checkout -b feature/AmazingFeature`)
3. Закоммитьте изменения (`git commit -m 'Add some AmazingFeature'`)
4. Push в ветку (`git push origin feature/AmazingFeature`)
5. Откройте Pull Request

## 👥 Команда



