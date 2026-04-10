import faust
import json
import logging
from datetime import datetime, timezone

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s [%(levelname)s] %(message)s"
)
logger = logging.getLogger(__name__)

# Faust приложение - ДОБАВЛЯЕМ ТОПИКИ В ПОДПИСКУ
app = faust.App(
    id="news-streams",
    broker="kafka://localhost:9092;localhost:9093;localhost:9094",
    value_serializer="raw",
    topic_partitions=3,  # Явно указываем количество партиций
)

# Топики
news_events_topic = app.topic("news-events", value_type=bytes)
enriched_topic = app.topic("news-enriched", value_type=bytes)

# Справочник источников
SOURCES_INFO = {
    "reuters": {"region": "Global", "reliability": "HIGH", "factCheck": 9.5},
    "bbc": {"region": "Europe", "reliability": "HIGH", "factCheck": 9.0},
    "cnn": {"region": "Americas", "reliability": "MEDIUM", "factCheck": 7.5},
    "aljazeera": {"region": "MiddleEast", "reliability": "HIGH", "factCheck": 8.5},
    "tass": {"region": "Russia", "reliability": "MEDIUM", "factCheck": 6.0},
}


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


@app.agent(news_events_topic)
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
            
            # Обогащаем данными
            enriched = {
                **event,
                "enriched": True,
                "sourceRegion": source_info["region"],
                "sourceReliability": source_info["reliability"],
                "credibilityScore": compute_credibility_score(event, source_info),
                "processedAt": datetime.now(timezone.utc).isoformat(),
            }
            
            logger.info(f"[TRANSFORM] {event['eventType']} | source={source_id} | credibility={enriched['credibilityScore']}")
            
            # Отправляем в news-enriched
            await enriched_topic.send(
                key=event.get('eventId', source_id).encode('utf-8'),
                value=json.dumps(enriched).encode('utf-8')
            )
            
        except Exception as e:
            logger.error(f"[TRANSFORM] Error: {e}")


if __name__ == "__main__":
    app.main()