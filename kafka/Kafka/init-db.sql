-- ─────────────────────────────────────────
-- Таблицы для мониторинга транспорта
-- Создаются автоматически при первом запуске
-- ─────────────────────────────────────────

-- Все входящие события (Kafka Connect пишет сюда)
CREATE TABLE IF NOT EXISTS transport_events (
    id          BIGSERIAL PRIMARY KEY,
    event_id    VARCHAR(36) NOT NULL UNIQUE,
    event_type  VARCHAR(50) NOT NULL,   -- TripCreated / LocationUpdated / CrashDetected
    timestamp   TIMESTAMPTZ NOT NULL,
    source      VARCHAR(100),
    version     VARCHAR(10),
    entry_id    VARCHAR(100),
    vehicle_id  VARCHAR(50),            -- ключ сообщения из Kafka
    payload     JSONB,
    created_at  TIMESTAMPTZ DEFAULT NOW()
);

-- Поездки
CREATE TABLE IF NOT EXISTS trips (
    id              BIGSERIAL PRIMARY KEY,
    trip_id         VARCHAR(36) NOT NULL UNIQUE,
    vehicle_id      VARCHAR(50) NOT NULL,
    driver_id       VARCHAR(50),
    start_location  VARCHAR(255),
    status          VARCHAR(20) DEFAULT 'active',
    created_at      TIMESTAMPTZ DEFAULT NOW()
);

-- Местоположения (высокочастотные события)
CREATE TABLE IF NOT EXISTS locations (
    id          BIGSERIAL PRIMARY KEY,
    vehicle_id  VARCHAR(50) NOT NULL,
    lat         DOUBLE PRECISION,
    lng         DOUBLE PRECISION,
    speed       DOUBLE PRECISION,
    heading     DOUBLE PRECISION,
    recorded_at TIMESTAMPTZ NOT NULL,
    created_at  TIMESTAMPTZ DEFAULT NOW()
);

-- Аварии
CREATE TABLE IF NOT EXISTS crashes (
    id          BIGSERIAL PRIMARY KEY,
    vehicle_id  VARCHAR(50) NOT NULL,
    lat         DOUBLE PRECISION,
    lng         DOUBLE PRECISION,
    severity    VARCHAR(20),           -- LOW / MEDIUM / HIGH / CRITICAL
    sensor_data JSONB,
    occurred_at TIMESTAMPTZ NOT NULL,
    created_at  TIMESTAMPTZ DEFAULT NOW()
);

-- Агрегации из Kafka Streams (результат оконных вычислений)
CREATE TABLE IF NOT EXISTS crash_aggregations (
    id              BIGSERIAL PRIMARY KEY,
    region          VARCHAR(100),
    window_start    TIMESTAMPTZ NOT NULL,
    window_end      TIMESTAMPTZ NOT NULL,
    crash_count     INTEGER DEFAULT 0,
    created_at      TIMESTAMPTZ DEFAULT NOW()
);

-- Vehicles — справочник для Kafka Streams KTable lookup
-- Connect Source читает отсюда и публикует в Kafka
CREATE TABLE IF NOT EXISTS vehicles (
    vehicle_id      VARCHAR(50) PRIMARY KEY,
    vehicle_type    VARCHAR(50),       -- BUS / TRUCK / CAR
    region          VARCHAR(100),
    driver_id       VARCHAR(50),
    updated_at      TIMESTAMPTZ DEFAULT NOW()
);

-- Тестовые данные для справочника
INSERT INTO vehicles (vehicle_id, vehicle_type, region) VALUES
    ('vehicle-001', 'BUS',   'North'),
    ('vehicle-002', 'TRUCK', 'South'),
    ('vehicle-003', 'CAR',   'East'),
    ('vehicle-004', 'BUS',   'West'),
    ('vehicle-005', 'TRUCK', 'North')
ON CONFLICT DO NOTHING;

-- Индексы для производительности
CREATE INDEX IF NOT EXISTS idx_events_vehicle   ON transport_events(vehicle_id);
CREATE INDEX IF NOT EXISTS idx_events_type      ON transport_events(event_type);
CREATE INDEX IF NOT EXISTS idx_locations_vehicle ON locations(vehicle_id);
CREATE INDEX IF NOT EXISTS idx_crashes_vehicle  ON crashes(vehicle_id);
