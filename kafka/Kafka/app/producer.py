import json
import uuid
import time
import random
import logging
from datetime import datetime, timezone
from confluent_kafka import Producer

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s [%(levelname)s] %(message)s"
)
logger = logging.getLogger(__name__)

# ─────────────────────────────────────────
# Конфигурация Producer
# ─────────────────────────────────────────
KAFKA_CONFIG = {
    "bootstrap.servers": "localhost:9092,localhost:9093,localhost:9094",
    # Ждём подтверждения от всех реплик — надёжная доставка
    "acks": "all",
    # Повторные попытки при ошибке
    "retries": 3,
    "retry.backoff.ms": 500,
    # Сжатие сообщений
    "compression.type": "gzip",
    # Батчинг —  сообщения 10мс перед отправкой
    "linger.ms": 10,
}

TOPIC = "transport-events"

# Тестовые транспортные средства
VEHICLES = [
    {"vehicleId": "vehicle-001", "type": "BUS",   "region": "North"},
    {"vehicleId": "vehicle-002", "type": "TRUCK", "region": "South"},
    {"vehicleId": "vehicle-003", "type": "CAR",   "region": "East"},
    {"vehicleId": "vehicle-004", "type": "BUS",   "region": "West"},
    {"vehicleId": "vehicle-005", "type": "TRUCK", "region": "North"},
]

DRIVERS = ["driver-001", "driver-002", "driver-003", "driver-004", "driver-005"]


# ─────────────────────────────────────────
# Единый формат события
# Все 3 типа используют эту функцию
# ─────────────────────────────────────────
def build_event(event_type: str, vehicle_id: str, payload: dict) -> dict:
    return {
        "eventId":   str(uuid.uuid4()),         # уникальный ID события
        "eventType": event_type,                 # тип события
        "timestamp": datetime.now(timezone.utc).isoformat(),
        "source":    "vehicle-service",          # источник
        "version":   "1.0",                      # версия схемы
        "entryId":   str(uuid.uuid4()),          # ID записи для идемпотентности
        "payload":   payload,                    # данные специфичные для типа
    }


# ─────────────────────────────────────────
# Тип 1: TripCreated
# Событие — началась новая поездка
# ─────────────────────────────────────────
def create_trip_event(vehicle: dict) -> tuple[str, dict]:
    payload = {
        "tripId":        str(uuid.uuid4()),
        "vehicleId":     vehicle["vehicleId"],
        "vehicleType":   vehicle["type"],
        "driverId":      random.choice(DRIVERS),
        "startLocation": {
            "lat": round(random.uniform(55.5, 56.0), 6),  # Москва примерно
            "lng": round(random.uniform(37.3, 37.9), 6),
        },
        "plannedDistance": round(random.uniform(5.0, 150.0), 1),
    }
    event = build_event("TripCreated", vehicle["vehicleId"], payload)
    return vehicle["vehicleId"], event


# ─────────────────────────────────────────
# Тип 2: LocationUpdated
# Событие — машина переместилась
# ─────────────────────────────────────────
def create_location_event(vehicle: dict) -> tuple[str, dict]:
    payload = {
        "vehicleId": vehicle["vehicleId"],
        "lat":       round(random.uniform(55.5, 56.0), 6),
        "lng":       round(random.uniform(37.3, 37.9), 6),
        "speed":     round(random.uniform(0.0, 120.0), 1),   # км/ч
        "heading":   round(random.uniform(0.0, 360.0), 1),   # градусы
        "fuel":      round(random.uniform(10.0, 100.0), 1),  # %
    }
    event = build_event("LocationUpdated", vehicle["vehicleId"], payload)
    return vehicle["vehicleId"], event


# ─────────────────────────────────────────
# Тип 3: CrashDetected
# Событие — зафиксирована авария
# ─────────────────────────────────────────
def create_crash_event(vehicle: dict) -> tuple[str, dict]:
    payload = {
        "vehicleId": vehicle["vehicleId"],
        "lat":       round(random.uniform(55.5, 56.0), 6),
        "lng":       round(random.uniform(37.3, 37.9), 6),
        "severity":  random.choice(["LOW", "MEDIUM", "HIGH", "CRITICAL"]),
        "sensorData": {
            "accelerometer": round(random.uniform(-20.0, 20.0), 2),
            "airbagDeployed": random.choice([True, False]),
            "engineStatus":   random.choice(["OK", "FAULT", "OFF"]),
        },
    }
    event = build_event("CrashDetected", vehicle["vehicleId"], payload)
    return vehicle["vehicleId"], event


# ─────────────────────────────────────────
# Callback — вызывается после доставки
# ─────────────────────────────────────────
def delivery_callback(err, msg):
    if err:
        logger.error(f"Delivery failed | key={msg.key()} | error={err}")
    else:
        logger.info(
            f"Delivered | topic={msg.topic()} | "
            f"partition={msg.partition()} | offset={msg.offset()} | "
            f"key={msg.key().decode()}"
        )


# ─────────────────────────────────────────
# Публикация события в Kafka
# key    = vehicleId
# value  = JSON событие
# headers= метаданные
# ─────────────────────────────────────────
def publish_event(producer: Producer, key: str, event: dict):
    # Метаданные сообщения — заголовки Kafka
    headers = {
        "correlationId":   str(uuid.uuid4()),
        "producerVersion": "1.0",
        "contentType":     "application/json",
        "eventType":       event["eventType"],
    }

    producer.produce(
        topic=TOPIC,
        key=key.encode("utf-8"),
        value=json.dumps(event).encode("utf-8"),
        headers=headers,
        on_delivery=delivery_callback,
    )


# ─────────────────────────────────────────
# Генераторы событий
# LocationUpdated — чаще всего (70%)
# TripCreated     — реже (20%)
# CrashDetected   — редко (10%)
# ─────────────────────────────────────────
EVENT_GENERATORS = [
    (create_location_event, 70),
    (create_trip_event,     20),
    (create_crash_event,    10),
]

def pick_random_event(vehicle: dict) -> tuple[str, dict]:
    generators = [g for g, w in EVENT_GENERATORS]
    weights    = [w for g, w in EVENT_GENERATORS]
    generator  = random.choices(generators, weights=weights, k=1)[0]
    return generator(vehicle)


# ─────────────────────────────────────────
# Главный цикл
# ─────────────────────────────────────────
def main():
    logger.info("Starting transport events producer...")
    logger.info(f"Topic: {TOPIC}")
    logger.info(f"Brokers: {KAFKA_CONFIG['bootstrap.servers']}")

    producer = Producer(KAFKA_CONFIG)

    try:
        while True:
            # Берём случайную машину
            vehicle = random.choice(VEHICLES)

            # Генерируем случайное событие
            key, event = pick_random_event(vehicle)

            # Публикуем в Kafka
            publish_event(producer, key, event)

            # Сбрасываем буфер каждые 10 сообщений
            producer.poll(0)

            logger.info(
                f"Produced | type={event['eventType']:<20} | "
                f"vehicle={key} | eventId={event['eventId'][:8]}..."
            )

            # Пауза между событиями (1 секунда)
            time.sleep(1)

    except KeyboardInterrupt:
        logger.info("Stopping producer...")
    finally:
        # Ждём доставки всех сообщений из буфера
        logger.info("Flushing remaining messages...")
        producer.flush()
        logger.info("Producer stopped.")


if __name__ == "__main__":
    main()