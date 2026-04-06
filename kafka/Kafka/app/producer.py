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

# Конфигурация Producer
KAFKA_CONFIG = {
    "bootstrap.servers": "localhost:9092,localhost:9093,localhost:9094",
    "acks": "all",
    "retries": 3,
    "retry.backoff.ms": 500,
    "compression.type": "gzip",
    "linger.ms": 10,
}

TOPIC = "news-events"

# Источники новостей
NEWS_SOURCES = [
    {"sourceId": "reuters", "region": "Global", "reliability": "HIGH"},
    {"sourceId": "bbc", "region": "Europe", "reliability": "HIGH"},
    {"sourceId": "cnn", "region": "Americas", "reliability": "MEDIUM"},
    {"sourceId": "aljazeera", "region": "MiddleEast", "reliability": "HIGH"},
    {"sourceId": "tass", "region": "Russia", "reliability": "MEDIUM"},
]

# Категории новостей
CATEGORIES = ["Politics", "Technology", "Sports", "Business", "Science", "Health", "Entertainment"]


def build_event(event_type: str, source_id: str, payload: dict) -> dict:
    """Единый формат события"""
    return {
        "eventId": str(uuid.uuid4()),
        "eventType": event_type,  # NewsPublished, NewsUpdated, NewsTrending
        "timestamp": datetime.now(timezone.utc).isoformat(),
        "source": source_id,
        "version": "1.0",
        "entryId": str(uuid.uuid4()),
        "payload": payload,
    }


def create_news_published_event(source: dict) -> tuple[str, dict]:
    """Тип 1: Новая новость опубликована"""
    category = random.choice(CATEGORIES)
    payload = {
        "articleId": str(uuid.uuid4()),
        "sourceId": source["sourceId"],
        "sourceRegion": source["region"],
        "category": category,
        "title": f"Breaking news in {category} from {source['sourceId']}",
        "content": f"This is a sample news content about {category}...",
        "author": f"Journalist_{random.randint(1, 20)}",
        "tags": [category.lower(), "breaking", "latest"],
        "views": random.randint(100, 10000),
        "likes": random.randint(0, 500),
        "shares": random.randint(0, 200),
        "reliability": source["reliability"],
    }
    event = build_event("NewsPublished", source["sourceId"], payload)
    return source["sourceId"], event


def create_news_updated_event(source: dict) -> tuple[str, dict]:
    """Тип 2: Новость обновлена (добавлены комментарии/правки)"""
    payload = {
        "articleId": str(uuid.uuid4()),
        "sourceId": source["sourceId"],
        "updateType": random.choice(["fact_check", "new_comments", "correction"]),
        "changes": {
            "updatedField": random.choice(["title", "content", "tags"]),
            "previousValue": "Old value",
            "newValue": "Updated value"
        },
        "updatedBy": f"editor_{random.randint(1, 10)}",
    }
    event = build_event("NewsUpdated", source["sourceId"], payload)
    return source["sourceId"], event


def create_news_trending_event(source: dict) -> tuple[str, dict]:
    """Тип 3: Новость стала трендовой"""
    payload = {
        "articleId": str(uuid.uuid4()),
        "sourceId": source["sourceId"],
        "trendingScore": round(random.uniform(80, 100), 1),
        "trendingRank": random.randint(1, 50),
        "engagement": {
            "viewsGrowth": f"+{random.randint(100, 1000)}%",
            "shareVelocity": random.randint(50, 500),
            "socialMentions": random.randint(100, 5000),
        },
        "region": source["region"],
    }
    event = build_event("NewsTrending", source["sourceId"], payload)
    return source["sourceId"], event


def delivery_callback(err, msg):
    if err:
        logger.error(f"Delivery failed | key={msg.key()} | error={err}")
    else:
        logger.info(
            f"Delivered | topic={msg.topic()} | "
            f"partition={msg.partition()} | offset={msg.offset()} | "
            f"key={msg.key().decode()}"
        )


def publish_event(producer: Producer, key: str, event: dict):
    headers = {
        "correlationId": str(uuid.uuid4()),
        "producerVersion": "1.0",
        "contentType": "application/json",
        "eventType": event["eventType"],
    }

    producer.produce(
        topic=TOPIC,
        key=key.encode("utf-8"),
        value=json.dumps(event).encode("utf-8"),
        headers=headers,
        on_delivery=delivery_callback,
    )


# Веса событий: NewsPublished (60%), NewsUpdated (30%), NewsTrending (10%)
EVENT_GENERATORS = [
    (create_news_published_event, 60),
    (create_news_updated_event, 30),
    (create_news_trending_event, 10),
]

def pick_random_event(source: dict) -> tuple[str, dict]:
    generators = [g for g, w in EVENT_GENERATORS]
    weights = [w for g, w in EVENT_GENERATORS]
    generator = random.choices(generators, weights=weights, k=1)[0]
    return generator(source)


def main():
    logger.info("Starting news aggregator producer...")
    logger.info(f"Topic: {TOPIC}")
    logger.info(f"Brokers: {KAFKA_CONFIG['bootstrap.servers']}")

    producer = Producer(KAFKA_CONFIG)

    try:
        while True:
            source = random.choice(NEWS_SOURCES)
            key, event = pick_random_event(source)
            publish_event(producer, key, event)
            producer.poll(0)

            logger.info(
                f"Produced | type={event['eventType']:<20} | "
                f"source={key} | eventId={event['eventId'][:8]}..."
            )

            time.sleep(1)

    except KeyboardInterrupt:
        logger.info("Stopping producer...")
    finally:
        logger.info("Flushing remaining messages...")
        producer.flush()
        logger.info("Producer stopped.")


if __name__ == "__main__":
    main()