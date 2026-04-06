import json
import time
import logging
from confluent_kafka import Consumer, Producer, KafkaError, KafkaException

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s [%(levelname)s] %(message)s"
)
logger = logging.getLogger(__name__)

# ─────────────────────────────────────────
# Конфигурация
# ─────────────────────────────────────────
BOOTSTRAP_SERVERS = "localhost:9092,localhost:9093,localhost:9094"

TOPIC_MAIN = "transport-events"   # основной топик
TOPIC_DLQ  = "transport-dlq"      # Dead Letter Queue — сюда летят проблемные события

MAX_RETRIES = 3  # сколько раз пробуем обработать событие перед DLQ


# ─────────────────────────────────────────
# Producer для DLQ
# Нужен чтобы писать проблемные события
# в отдельный топик transport-dlq
# ─────────────────────────────────────────
dlq_producer = Producer({
    "bootstrap.servers": BOOTSTRAP_SERVERS,
    "acks": "all",
})

def send_to_dlq(event: dict, reason: str):
    """Отправить событие в Dead Letter Queue"""
    dlq_payload = {
        "originalEvent": event,
        "reason":        reason,
        "failedAt":      time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
    }
    dlq_producer.produce(
        topic=TOPIC_DLQ,
        key=event.get("payload", {}).get("vehicleId", "unknown").encode("utf-8"),
        value=json.dumps(dlq_payload).encode("utf-8"),
    )
    dlq_producer.flush()
    logger.warning(f"Sent to DLQ | eventId={event.get('eventId')} | reason={reason}")


# ─────────────────────────────────────────
# Обработка события с повторными попытками
# ─────────────────────────────────────────
def process_with_retry(event: dict, handler, max_retries: int = MAX_RETRIES):
    """
    Пробуем обработать событие max_retries раз.
    Если все попытки провалились — отправляем в DLQ.
    """
    last_error = None

    for attempt in range(1, max_retries + 1):
        try:
            handler(event)
            return True  # успех
        except Exception as e:
            last_error = e
            logger.warning(
                f"Attempt {attempt}/{max_retries} failed | "
                f"eventId={event.get('eventId')} | error={e}"
            )
            if attempt < max_retries:
                time.sleep(0.5 * attempt)  # экспоненциальная задержка

    # Все попытки провалились → DLQ
    send_to_dlq(event, str(last_error))
    return False


# ═══════════════════════════════════════════════════════
# CONSUMER GROUP 1: trip-processor
# Обрабатывает ВСЕ события
# Использует РУЧНОЙ commit offset
# ═══════════════════════════════════════════════════════
def handle_trip_processor(event: dict):
    """
    Обработчик для group 1.
    """
    event_type = event.get("eventType")
    payload    = event.get("payload", {})
    vehicle_id = payload.get("vehicleId", "unknown")

    if event_type == "TripCreated":
        logger.info(
            f"[GROUP-1] TripCreated | vehicle={vehicle_id} | "
            f"tripId={payload.get('tripId')} | "
            f"driver={payload.get('driverId')}"
        )

    elif event_type == "LocationUpdated":
        logger.info(
            f"[GROUP-1] LocationUpdated | vehicle={vehicle_id} | "
            f"lat={payload.get('lat')} | lng={payload.get('lng')} | "
            f"speed={payload.get('speed')} km/h"
        )

    elif event_type == "CrashDetected":
        logger.info(
            f"[GROUP-1] CrashDetected | vehicle={vehicle_id} | "
            f"severity={payload.get('severity')} | "
            f"lat={payload.get('lat')} | lng={payload.get('lng')}"
        )

    else:
        # Неизвестный тип события — это ошибка
        raise ValueError(f"Unknown event type: {event_type}")


def run_trip_processor():
    """
    Consumer Group 1: trip-processor
    РУЧНОЙ commit — коммитим только после успешной обработки.
    Это гарантирует что событие не потеряется при сбое.
    """
    consumer = Consumer({
        "bootstrap.servers":  BOOTSTRAP_SERVERS,
        "group.id":           "trip-processor",       # имя группы
        "auto.offset.reset":  "earliest",             # читать с начала если нет офсета
        "enable.auto.commit": False,                  # РУЧНОЙ commit
    })

    consumer.subscribe([TOPIC_MAIN])
    logger.info("Consumer Group 1 [trip-processor] started | manual commit | all events")

    try:
        while True:
            msg = consumer.poll(timeout=1.0)

            if msg is None:
                continue

            if msg.error():
                if msg.error().code() == KafkaError._PARTITION_EOF:
                    # Дошли до конца партиции — это нормально
                    continue
                raise KafkaException(msg.error())

            try:
                # Десериализуем JSON
                event = json.loads(msg.value().decode("utf-8"))

                # Обрабатываем с повторными попытками
                success = process_with_retry(event, handle_trip_processor)

                # РУЧНОЙ commit — только после обработки
                # Если упадём до этой строки — событие будет перечитано
                consumer.commit(message=msg, asynchronous=False)

                if success:
                    logger.debug(
                        f"[GROUP-1] Committed offset={msg.offset()} "
                        f"partition={msg.partition()}"
                    )

            except json.JSONDecodeError as e:
                logger.error(f"[GROUP-1] Invalid JSON: {e}")
                # Коммитим даже битое сообщение чтобы не застрять
                consumer.commit(message=msg, asynchronous=False)

    except KeyboardInterrupt:
        logger.info("Consumer Group 1 stopping...")
    finally:
        consumer.close()
        logger.info("Consumer Group 1 stopped.")


# ═══════════════════════════════════════════════════════
# CONSUMER GROUP 2: alert-processor
# Обрабатывает ТОЛЬКО CrashDetected
# Использует АВТОМАТИЧЕСКИЙ commit offset
# Логика: отправить алерт при аварии
# ═══════════════════════════════════════════════════════
def handle_alert_processor(event: dict):
    """
    Обработчик для group 2.
    Реагирует только на аварии — имитирует отправку алерта.
    """
    event_type = event.get("eventType")

    # Пропускаем всё кроме аварий
    if event_type != "CrashDetected":
        return

    payload    = event.get("payload", {})
    vehicle_id = payload.get("vehicleId", "unknown")
    severity   = payload.get("severity", "UNKNOWN")
    sensor     = payload.get("sensorData", {})

    # Имитация отправки алерта (webhook/email/SMS)
    logger.warning(
        f"[GROUP-2] *** ALERT *** CrashDetected | "
        f"vehicle={vehicle_id} | severity={severity} | "
        f"airbag={sensor.get('airbagDeployed')} | "
        f"engine={sensor.get('engineStatus')} | "
        f"lat={payload.get('lat')} | lng={payload.get('lng')}"
    )

    # При CRITICAL severity — дополнительный алерт
    if severity == "CRITICAL":
        logger.error(
            f"[GROUP-2] !!! CRITICAL CRASH !!! "
            f"vehicle={vehicle_id} — emergency services required!"
        )


def run_alert_processor():
    """
    Consumer Group 2: alert-processor
    at-least-once семантика допустима для уведомлений.
    """
    consumer = Consumer({
        "bootstrap.servers":     BOOTSTRAP_SERVERS,
        "group.id":              "alert-processor",    # отдельная группа
        "auto.offset.reset":     "earliest",
        "enable.auto.commit":    True,                 # АВТОМАТИЧЕСКИЙ commit
        "auto.commit.interval.ms": 5000,               # коммитим каждые 5 секунд
    })

    consumer.subscribe([TOPIC_MAIN])
    logger.info("Consumer Group 2 [alert-processor] started | auto commit | crashes only")

    try:
        while True:
            msg = consumer.poll(timeout=1.0)

            if msg is None:
                continue

            if msg.error():
                if msg.error().code() == KafkaError._PARTITION_EOF:
                    continue
                raise KafkaException(msg.error())

            try:
                event = json.loads(msg.value().decode("utf-8"))

                # Обрабатываем с повторными попытками
                process_with_retry(event, handle_alert_processor)

            except json.JSONDecodeError as e:
                logger.error(f"[GROUP-2] Invalid JSON: {e}")

    except KeyboardInterrupt:
        logger.info("Consumer Group 2 stopping...")
    finally:
        consumer.close()
        logger.info("Consumer Group 2 stopped.")


# ─────────────────────────────────────────
# Запуск — выбираем какую группу запустить
# ─────────────────────────────────────────
if __name__ == "__main__":
    import sys

    if len(sys.argv) < 2:
        print("Usage:")
        print("  python consumer.py group1   — trip-processor (manual commit, all events)")
        print("  python consumer.py group2   — alert-processor (auto commit, crashes only)")
        sys.exit(1)

    group = sys.argv[1]

    if group == "group1":
        run_trip_processor()
    elif group == "group2":
        run_alert_processor()
    else:
        print(f"Unknown group: {group}")
        print("Use: group1 or group2")
        sys.exit(1)