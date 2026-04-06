import faust
import json
import logging
from datetime import datetime, timezone

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s [%(levelname)s] %(message)s"
)
logger = logging.getLogger(__name__)

# ─────────────────────────────────────────
# Faust приложение
# id        — уникальное имя приложения
# broker    — Kafka брокеры
# ─────────────────────────────────────────
app = faust.App(
    id="transport-streams",
    broker="kafka://localhost:9092;localhost:9093;localhost:9094",
    value_serializer="raw",
)

# ─────────────────────────────────────────
# Топики
# ─────────────────────────────────────────
# Входной топик — читаем отсюда
transport_events_topic = app.topic(
    "transport-events",
    value_type=bytes,
)

# Выходной топик 1 — результат трансформации
enriched_topic = app.topic(
    "transport-enriched",
    value_type=bytes,
)

# Выходной топик 2 — результат агрегации и оконного вычисления
aggregated_topic = app.topic(
    "crash-aggregated",
    value_type=bytes,
)

# ─────────────────────────────────────────
# Справочник машин (имитация KTable lookup)
# В реальном проекте это читалось бы из БД
# ─────────────────────────────────────────
VEHICLE_INFO = {
    "vehicle-001": {"vehicleType": "BUS",   "region": "North", "capacity": 50},
    "vehicle-002": {"vehicleType": "TRUCK", "region": "South", "capacity": 10},
    "vehicle-003": {"vehicleType": "CAR",   "region": "East",  "capacity": 5},
    "vehicle-004": {"vehicleType": "BUS",   "region": "West",  "capacity": 50},
    "vehicle-005": {"vehicleType": "TRUCK", "region": "North", "capacity": 10},
}

# ─────────────────────────────────────────
# Таблица для агрегации
# Хранит количество аварий по регионам
# Персистентная — сохраняется в Kafka топик
# ─────────────────────────────────────────
crash_counts = app.Table(
    "crash-counts-by-region",
    default=int,
    help="Aggregation: crash count per region",
)

# ─────────────────────────────────────────
# Таблица для оконного вычисления
# Tumbling window — 60 секунд
# Считаем аварии за каждую минуту
# ─────────────────────────────────────────
crash_window = app.Table(
    "crash-window-counts",
    default=int,
    help="Windowed: crash count per minute window",
).tumbling(60.0, expires=300.0)  # окно 60 сек, хранить 5 минут


# ═══════════════════════════════════════════════════════
# ТРАНСФОРМАЦИЯ
# Читаем transport-events → обогащаем данными о машине
# → пишем в transport-enriched
#
# Что добавляем: vehicleType, region, capacity, riskLevel
# ═══════════════════════════════════════════════════════
@app.agent(transport_events_topic, sink=[enriched_topic])
async def enrich_events(events):
    async for event_bytes in events:
        try:
            event = json.loads(event_bytes.decode("utf-8"))
            payload = event.get("payload", {})
            vehicle_id = payload.get("vehicleId", "unknown")

            # Получаем доп. инфо о машине из справочника
            vehicle_info = VEHICLE_INFO.get(vehicle_id, {
                "vehicleType": "UNKNOWN",
                "region": "UNKNOWN",
                "capacity": 0,
            })

            # ТРАНСФОРМАЦИЯ — добавляем новые поля к событию
            enriched = {
                **event,
                "enriched": True,
                "vehicleType": vehicle_info["vehicleType"],
                "region":      vehicle_info["region"],
                "capacity":    vehicle_info["capacity"],
                # Вычисляем уровень риска на основе типа события и машины
                "riskLevel":   compute_risk_level(event, vehicle_info),
            }

            logger.info(
                f"[TRANSFORM] {event['eventType']} | "
                f"vehicle={vehicle_id} | "
                f"region={vehicle_info['region']} | "
                f"risk={enriched['riskLevel']}"
            )
            # Отдаёт результат и ждёт следующее событие
            yield json.dumps(enriched).encode("utf-8")

        except Exception as e:
            logger.error(f"[TRANSFORM] Error: {e}")


def compute_risk_level(event: dict, vehicle_info: dict) -> str:
    """
    Трансформация: вычисляем уровень риска события.
    Логика зависит от типа события и типа транспорта.
    """
    event_type   = event.get("eventType")
    payload      = event.get("payload", {})
    vehicle_type = vehicle_info.get("vehicleType", "UNKNOWN")

    if event_type == "CrashDetected":
        severity = payload.get("severity", "LOW")
        # Автобусы и грузовики — выше риск из-за количества людей
        if vehicle_type in ("BUS", "TRUCK") and severity in ("HIGH", "CRITICAL"):
            return "CRITICAL"
        if severity == "CRITICAL":
            return "HIGH"
        if severity == "HIGH":
            return "MEDIUM"
        return "LOW"

    elif event_type == "LocationUpdated":
        speed = payload.get("speed", 0)
        if speed > 100:
            return "MEDIUM"   # превышение скорости
        return "LOW"

    return "LOW"


# ═══════════════════════════════════════════════════════
# АГРЕГАЦИЯ + ОКОННОЕ ВЫЧИСЛЕНИЕ
# Читаем transport-enriched → считаем аварии
# → пишем результат в crash-aggregated
#
# Агрегация:    общий счётчик аварий по регионам
# Оконное:      аварии за последние 60 секунд
# ═══════════════════════════════════════════════════════
@app.agent(enriched_topic)
async def aggregate_crashes(events):
    async for event_bytes in events:
        try:
            event = json.loads(event_bytes.decode("utf-8"))

            # Обрабатываем только аварии
            if event.get("eventType") != "CrashDetected":
                continue

            region     = event.get("region", "UNKNOWN")
            vehicle_id = event.get("payload", {}).get("vehicleId", "unknown")
            severity   = event.get("payload", {}).get("severity", "LOW")

            # АГРЕГАЦИЯ — общий счётчик по регионам (без окна)
            crash_counts[region] += 1
            total = crash_counts[region]

            # ОКОННОЕ ВЫЧИСЛЕНИЕ — счётчик за текущее окно 60 сек
            crash_window[region] += 1
            window_count = crash_window[region].current()

            logger.info(
                f"[AGGREGATE] CrashDetected | "
                f"region={region} | vehicle={vehicle_id} | "
                f"severity={severity} | "
                f"total_crashes={total} | "
                f"window_crashes(60s)={window_count}"
            )

            # Формируем результат с явной схемой для JDBC Sink
            result = {
                "schema": {
                    "type": "struct",
                    "optional": False,
                    "fields": [
                        {"field": "eventType",     "type": "string", "optional": True},
                        {"field": "timestamp",     "type": "string", "optional": True},
                        {"field": "region",        "type": "string", "optional": True},
                        {"field": "vehicleId",     "type": "string", "optional": True},
                        {"field": "severity",      "type": "string", "optional": True},
                        {"field": "totalCrashes",  "type": "int32",  "optional": True},
                        {"field": "windowCrashes", "type": "int32",  "optional": True},
                        {"field": "windowSeconds", "type": "int32",  "optional": True},
                    ]
                },
                "payload": {
                    "eventType":      "CrashAggregation",
                    "timestamp":      datetime.now(timezone.utc).isoformat(),
                    "region":         region,
                    "vehicleId":      vehicle_id,
                    "severity":       severity,
                    "totalCrashes":   total,
                    "windowCrashes":  window_count,
                    "windowSeconds":  60,
                }
            }
            # Отправляем результат в crash-aggregated
            await aggregated_topic.send(
                key=region.encode("utf-8"),
                value=json.dumps(result).encode("utf-8"),
            )

        except Exception as e:
            logger.error(f"[AGGREGATE] Error: {e}")


# ─────────────────────────────────────────
# Запуск
# ─────────────────────────────────────────
if __name__ == "__main__":
    app.main()