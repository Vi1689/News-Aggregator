import faust
import json
import logging
from datetime import datetime, timezone

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s [%(levelname)s] %(message)s"
)
logger = logging.getLogger(__name__)

app = faust.App(
    id="news-streams",
    broker="kafka://localhost:9092;localhost:9093;localhost:9094",
    value_serializer="raw",
    topic_partitions=3,
)

# Топики
news_events_topic = app.topic("news-events", value_type=bytes)
enriched_topic = app.topic("news-enriched", value_type=bytes)
windowed_topic = app.topic("news-windowed", value_type=bytes)

# Справочник источников
SOURCES_INFO = {
    "reuters": {"region": "Global", "reliability": "HIGH", "factCheck": 9.5},
    "bbc": {"region": "Europe", "reliability": "HIGH", "factCheck": 9.0},
    "cnn": {"region": "Americas", "reliability": "MEDIUM", "factCheck": 7.5},
    "aljazeera": {"region": "MiddleEast", "reliability": "HIGH", "factCheck": 8.5},
    "tass": {"region": "Russia", "reliability": "MEDIUM", "factCheck": 6.0},
}

# ============================================================================
# ОКОННЫЕ ТАБЛИЦЫ (WINDOWS)
# ============================================================================

trending_tumbling_window = app.Table(
    "trending-tumbling-window",
    default=int,
).tumbling(60.0, expires=300.0)

trending_hopping_window = app.Table(
    "trending-hopping-window",
    default=int,
).hopping(60.0, 20.0, expires=300.0)

category_tumbling_window = app.Table(
    "category-tumbling-window",
    default=int,
).tumbling(30.0, expires=180.0)

total_events_window = app.Table(
    "total-events-tumbling-window",
    default=int,
).tumbling(60.0, expires=300.0)


def compute_credibility_score(event: dict, source_info: dict) -> float:
    base_score = source_info.get("factCheck", 5.0)
    event_type = event.get("eventType")
    if event_type == "NewsPublished":
        return base_score
    elif event_type == "NewsUpdated":
        update_type = event.get("payload", {}).get("updateType")
        if update_type == "fact_check":
            return base_score + 1.0
        return base_score - 0.5
    elif event_type == "NewsTrending":
        trending_score = event.get("payload", {}).get("trendingScore", 0)
        return min(10.0, base_score + (trending_score - 80) / 20)
    return base_score


# ============================================================================
# ОСНОВНОЙ ПРОЦЕССОР: ОБОГАЩЕНИЕ ДАННЫХ
# ============================================================================

@app.agent(news_events_topic, sink=[enriched_topic])
async def enrich_news(events):
    async for event_bytes in events:
        try:
            event = json.loads(event_bytes.decode("utf-8"))
            source_id = event.get("source", "unknown")
            source_info = SOURCES_INFO.get(source_id, {
                "region": "Unknown",
                "reliability": "LOW",
                "factCheck": 5.0
            })
            
            enriched = {
                **event,
                "enriched": True,
                "sourceRegion": source_info["region"],
                "sourceReliability": source_info["reliability"],
                "credibilityScore": compute_credibility_score(event, source_info),
                "processedAt": datetime.now(timezone.utc).isoformat(),
            }
            
            logger.info(f"[TRANSFORM] {event['eventType']} | source={source_id} | credibility={enriched['credibilityScore']}")
            
            yield json.dumps(enriched).encode('utf-8')
            
        except Exception as e:
            logger.error(f"[TRANSFORM] Error: {e}")


# ============================================================================
# ПРОЦЕССОР С ОКНАМИ: АНАЛИЗ ТРЕНДОВ
# ============================================================================

@app.agent(enriched_topic)
async def analyze_windows(events):
    async for event_bytes in events:
        try:
            if isinstance(event_bytes, bytes):
                event = json.loads(event_bytes.decode("utf-8"))
            else:
                event = event_bytes
            
            event_type = event.get("eventType")
            source_id = event.get("source", "unknown")
            payload = event.get("payload", {})
            category = payload.get("category", "Unknown")
            
            # TUMBLING WINDOW (60 секунд)
            if event_type == "NewsTrending":
                trending_tumbling_window[source_id] += 1
                # Просто логируем, не пытаемся прочитать значение
                logger.info(f"[TUMBLING WINDOW 60s] source={source_id} | incremented")
            
            # HOPPING WINDOW (60 сек, шаг 20 сек)
            if event_type == "NewsTrending":
                trending_hopping_window[source_id] += 1
                logger.info(f"[HOPPING WINDOW 60s/20s] source={source_id} | incremented")
            
            # TOTAL EVENTS WINDOW (60 секунд)
            total_events_window["total"] += 1
            logger.info(f"[TOTAL EVENTS WINDOW 60s] total_events incremented")
            
            # CATEGORY WINDOW (30 секунд)
            if event_type == "NewsPublished":
                category_tumbling_window[category] += 1
                logger.info(f"[CATEGORY WINDOW 30s] category={category} | incremented")
            
        except Exception as e:
            logger.error(f"[WINDOW ANALYSIS] Error: {e}")


# ============================================================================
# ДОПОЛНИТЕЛЬНЫЙ ПРОЦЕССОР: АГРЕГАЦИЯ С ОКНАМИ
# ============================================================================

@app.agent(windowed_topic)
async def aggregate_window_stats(events):
    async for event_bytes in events:
        try:
            if isinstance(event_bytes, bytes):
                event = json.loads(event_bytes.decode("utf-8"))
            else:
                event = event_bytes
            
            logger.info(f"[AGGREGATE] Received windowed event: {event.get('eventId')}")
        except Exception as e:
            logger.error(f"[AGGREGATE WINDOW STATS] Error: {e}")


# ============================================================================
# КОМАНДА ДЛЯ ПРОСМОТРА ТЕКУЩИХ ЗНАЧЕНИЙ ОКОН
# ============================================================================

@app.command()
async def show_windows():
    print("\n=== ТЕКУЩИЕ ЗНАЧЕНИЯ ОКОН ===\n")
    
    print("1. TUMBLING WINDOW (60s):")
    for key, value in trending_tumbling_window.items():
        print(f"   {key}: {value}")
    
    print("\n2. HOPPING WINDOW (60s/20s):")
    for key, value in trending_hopping_window.items():
        print(f"   {key}: {value}")
    
    print(f"\n3. TOTAL EVENTS WINDOW (60s): {total_events_window.get('total', 0)}")
    
    print("\n4. CATEGORY TUMBLING WINDOW (30s):")
    for key, value in category_tumbling_window.items():
        print(f"   {key}: {value}")
    
    print("\n================================\n")


if __name__ == "__main__":
    app.main()