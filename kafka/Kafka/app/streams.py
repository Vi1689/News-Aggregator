import faust
import json
import logging
from datetime import datetime, timezone
from typing import Dict, Any

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s [%(levelname)s] %(message)s"
)
logger = logging.getLogger(__name__)

# Faust приложение
app = faust.App(
    id="news-streams",
    broker="kafka://localhost:9092;localhost:9093;localhost:9094",
    value_serializer="raw",
)

# Топики
news_events_topic = app.topic("news-events", value_type=bytes)
enriched_topic = app.topic("news-enriched", value_type=bytes)
aggregated_topic = app.topic("news-aggregated", value_type=bytes)

# Справочник источников (имитация KTable lookup)
SOURCES_INFO = {
    "reuters": {"region": "Global", "reliability": "HIGH", "factCheck": 9.5},
    "bbc": {"region": "Europe", "reliability": "HIGH", "factCheck": 9.0},
    "cnn": {"region": "Americas", "reliability": "MEDIUM", "factCheck": 7.5},
    "aljazeera": {"region": "MiddleEast", "reliability": "HIGH", "factCheck": 8.5},
    "tass": {"region": "Russia", "reliability": "MEDIUM", "factCheck": 6.0},
}

# Таблицы для агрегации
category_counts = app.Table("news-counts-by-category", default=int)
region_counts = app.Table("news-counts-by-region", default=int)

# Оконное вычисление (Tumbling window - 60 секунд)
trending_window = app.Table(
    "trending-news-window",
    default=int,
    help="Windowed: trending news per minute"
).tumbling(60.0, expires=300.0)


# ТРАНСФОРМАЦИЯ: Обогащение новостей
def compute_credibility_score(event: dict, source_info: dict) -> float:
    """Вычисление credibility score на основе источника и типа события"""
    base_score = source_info.get("factCheck", 5.0)
    
    if event.get("eventType") == "NewsPublished":
        return base_score
    elif event.get("eventType") == "NewsUpdated":
        update_type = event.get("payload", {}).get("updateType")
        if update_type == "fact_check":
            return base_score + 1.0
        return base_score - 0.5
    elif event.get("eventType") == "NewsTrending":
        trending_score = event.get("payload", {}).get("trendingScore", 0)
        return min(10.0, base_score + (trending_score - 80) / 20)
    return base_score


@app.agent(news_events_topic, sink=[enriched_topic])
async def enrich_news(events):
    async for event_bytes in events:
        try:
            event = json.loads(event_bytes.decode("utf-8"))
            source_id = event.get("source", "unknown")
            
            source_info = SOURCES_INFO.get(source_id, {
                "region": "Unknown",
                "reliability": "LOW",
                "factCheck": 5.0,
            })
            
            # ТРАНСФОРМАЦИЯ: добавляем новые поля
            enriched = {
                **event,
                "enriched": True,
                "sourceRegion": source_info["region"],
                "sourceReliability": source_info["reliability"],
                "credibilityScore": compute_credibility_score(event, source_info),
                "processedAt": datetime.now(timezone.utc).isoformat(),
            }
            
            logger.info(
                f"[TRANSFORM] {event['eventType']} | "
                f"source={source_id} | "
                f"region={source_info['region']} | "
                f"credibility={enriched['credibilityScore']}"
            )
            
            yield json.dumps(enriched).encode("utf-8")
            
        except Exception as e:
            logger.error(f"[TRANSFORM] Error: {e}")


# АГРЕГАЦИЯ + ОКОННОЕ ВЫЧИСЛЕНИЕ
@app.agent(enriched_topic)
async def aggregate_news(events):
    async for event_bytes in events:
        try:
            event = json.loads(event_bytes.decode("utf-8"))
            
            # Агрегируем только опубликованные новости
            if event.get("eventType") == "NewsPublished":
                payload = event.get("payload", {})
                category = payload.get("category", "Unknown")
                source_region = event.get("sourceRegion", "Unknown")
                
                # АГРЕГАЦИЯ 1: по категориям
                category_counts[category] += 1
                total_by_category = category_counts[category]
                
                # АГРЕГАЦИЯ 2: по регионам
                region_counts[source_region] += 1
                total_by_region = region_counts[source_region]
                
                logger.info(
                    f"[AGGREGATE] NewsPublished | "
                    f"category={category} | total={total_by_category} | "
                    f"region={source_region} | region_total={total_by_region}"
                )
            
            # ОКОННОЕ ВЫЧИСЛЕНИЕ: трендовые новости за минуту
            if event.get("eventType") == "NewsTrending":
                source_id = event.get("source", "unknown")
                trending_window[source_id] += 1
                window_count = trending_window[source_id].current()
                
                logger.info(
                    f"[WINDOW] Trending news | "
                    f"source={source_id} | "
                    f"window_count(60s)={window_count}"
                )
            
            # Отправляем агрегированный результат
            result = {
                "schema": {
                    "type": "struct",
                    "optional": False,
                    "fields": [
                        {"field": "timestamp", "type": "string", "optional": True},
                        {"field": "eventType", "type": "string", "optional": True},
                        {"field": "category", "type": "string", "optional": True},
                        {"field": "region", "type": "string", "optional": True},
                        {"field": "totalByCategory", "type": "int32", "optional": True},
                        {"field": "totalByRegion", "type": "int32", "optional": True},
                        {"field": "trendingCount", "type": "int32", "optional": True},
                    ]
                },
                "payload": {
                    "timestamp": datetime.now(timezone.utc).isoformat(),
                    "eventType": "NewsAggregation",
                    "category": event.get("payload", {}).get("category", "N/A"),
                    "region": event.get("sourceRegion", "N/A"),
                    "totalByCategory": category_counts.get(event.get("payload", {}).get("category", "Unknown"), 0),
                    "totalByRegion": region_counts.get(event.get("sourceRegion", "Unknown"), 0),
                    "trendingCount": trending_window.get(event.get("source", "unknown"), 0),
                }
            }
            
            await aggregated_topic.send(
                key=event.get("source", "unknown").encode("utf-8"),
                value=json.dumps(result).encode("utf-8"),
            )
            
        except Exception as e:
            logger.error(f"[AGGREGATE] Error: {e}")


if __name__ == "__main__":
    app.main()