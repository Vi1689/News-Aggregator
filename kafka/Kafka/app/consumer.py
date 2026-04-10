import json
import time
import logging
from confluent_kafka import Consumer, Producer, KafkaError, KafkaException

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s [%(levelname)s] %(message)s"
)
logger = logging.getLogger(__name__)

BOOTSTRAP_SERVERS = "localhost:9092,localhost:9093,localhost:9094"
TOPIC_MAIN = "news-events"
TOPIC_DLQ = "news-dlq"

MAX_RETRIES = 2

# Producer для DLQ
dlq_producer = Producer({
    "bootstrap.servers": BOOTSTRAP_SERVERS,
    "acks": "all",
})

def send_to_dlq(event: dict, reason: str):
    dlq_payload = {
        "originalEvent": event,
        "reason": reason,
        "failedAt": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
    }
    dlq_producer.produce(
        topic=TOPIC_DLQ,
        key=event.get("source", "unknown").encode("utf-8"),
        value=json.dumps(dlq_payload).encode("utf-8"),
    )
    dlq_producer.flush()
    logger.warning(f"Sent to DLQ | eventId={event.get('eventId')} | reason={reason}")


def process_with_retry(event: dict, handler, max_retries: int = MAX_RETRIES):
    last_error = None
    for attempt in range(1, max_retries + 1):
        try:
            handler(event)
            return True
        except Exception as e:
            last_error = e
            logger.warning(f"Attempt {attempt}/{max_retries} failed | error={e}")
            if attempt < max_retries:
                time.sleep(0.5 * attempt)
    send_to_dlq(event, str(last_error))
    return False


# ═══════════════════════════════════════════════════════
# CONSUMER GROUP 1: news-analyzer (manual commit)
# Анализирует все новости, считает метрики
# ═══════════════════════════════════════════════════════
def handle_news_analyzer(event: dict):
    event_type = event.get("eventType")
    payload = event.get("payload", {})
    source = event.get("source", "unknown")
    
    if event_type == "NewsPublished":
        logger.info(
            f"[GROUP-1] NewsAnalyzer | source={source} | "
            f"category={payload.get('category')} | "
            f"title={payload.get('title')[:50]}... | "
            f"views={payload.get('views')} | likes={payload.get('likes')}"
        )
        
        # Анализируем engagement rate
        views = payload.get("views", 0)
        likes = payload.get("likes", 0)
        shares = payload.get("shares", 0)
        
        engagement_rate = (likes + shares) / max(views, 1) * 100
        if engagement_rate > 10:
            logger.info(f"[GROUP-1] High engagement detected! Rate={engagement_rate:.1f}%")
            
    elif event_type == "NewsUpdated":
        logger.info(
            f"[GROUP-1] NewsAnalyzer | source={source} | "
            f"updateType={payload.get('updateType')} | "
            f"updatedBy={payload.get('updatedBy')}"
        )
        
    elif event_type == "NewsTrending":
        logger.info(
            f"[GROUP-1] NewsAnalyzer | source={source} | "
            f"trendingRank={payload.get('trendingRank')} | "
            f"trendingScore={payload.get('trendingScore')}"
        )
        
    else:
        raise ValueError(f"Unknown event type: {event_type}")


def run_news_analyzer():
    """Consumer Group 1: РУЧНОЙ commit"""
    consumer = Consumer({
        "bootstrap.servers": BOOTSTRAP_SERVERS,
        "group.id": "news-analyzer",
        "auto.offset.reset": "earliest",
        "enable.auto.commit": False,
    })
    
    consumer.subscribe([TOPIC_MAIN])
    logger.info("Consumer Group 1 [news-analyzer] started | manual commit | all events")
    
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
                success = process_with_retry(event, handle_news_analyzer)
                consumer.commit(message=msg, asynchronous=False)
                if success:
                    logger.debug(f"[GROUP-1] Committed offset={msg.offset()}")
            except json.JSONDecodeError as e:
                logger.error(f"[GROUP-1] Invalid JSON: {e}")
                consumer.commit(message=msg, asynchronous=False)
                
    except KeyboardInterrupt:
        logger.info("Consumer Group 1 stopping...")
    finally:
        consumer.close()
        logger.info("Consumer Group 1 stopped.")


# ═══════════════════════════════════════════════════════
# CONSUMER GROUP 2: alert-processor (auto commit)
# Отправляет алерты на важные новости
# ═══════════════════════════════════════════════════════
def handle_alert_processor(event: dict):
    event_type = event.get("eventType")
    payload = event.get("payload", {})
    source = event.get("source", "unknown")
    
    # Реагируем на все типы, но с разными приоритетами
    if event_type == "NewsPublished":
        category = payload.get("category")
        reliability = payload.get("reliability", "LOW")
        
        if category in ["Politics", "Business"] and reliability == "HIGH":
            logger.warning(
                f"[GROUP-2] *** ALERT *** Important news from {source} | "
                f"category={category} | reliability={reliability}"
            )
        else:
            logger.info(f"[GROUP-2] NewsPublished | source={source} | category={category}")
            
    elif event_type == "NewsTrending":
        trending_rank = payload.get("trendingRank", 100)
        if trending_rank <= 10:
            logger.error(
                f"[GROUP-2] !!! CRITICAL !!! News in TOP-10 trending! | "
                f"source={source} | rank={trending_rank}"
            )
        else:
            logger.info(f"[GROUP-2] NewsTrending | source={source} | rank={trending_rank}")
            
    elif event_type == "NewsUpdated":
        update_type = payload.get("updateType")
        if update_type == "fact_check":
            logger.info(f"[GROUP-2] Fact check update | source={source}")


def run_alert_processor():
    """Consumer Group 2: АВТОМАТИЧЕСКИЙ commit"""
    consumer = Consumer({
        "bootstrap.servers": BOOTSTRAP_SERVERS,
        "group.id": "alert-processor",
        "auto.offset.reset": "earliest",
        "enable.auto.commit": True,
        "auto.commit.interval.ms": 5000,
    })
    
    consumer.subscribe([TOPIC_MAIN])
    logger.info("Consumer Group 2 [alert-processor] started | auto commit | all events")
    
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
                process_with_retry(event, handle_alert_processor)
            except json.JSONDecodeError as e:
                logger.error(f"[GROUP-2] Invalid JSON: {e}")
                
    except KeyboardInterrupt:
        logger.info("Consumer Group 2 stopping...")
    finally:
        consumer.close()
        logger.info("Consumer Group 2 stopped.")


if __name__ == "__main__":
    import sys
    
    if len(sys.argv) < 2:
        print("Usage:")
        print("  python consumer.py group1   — news-analyzer (manual commit)")
        print("  python consumer.py group2   — alert-processor (auto commit)")
        sys.exit(1)
    
    group = sys.argv[1]
    
    if group == "group1":
        run_news_analyzer()
    elif group == "group2":
        run_alert_processor()
    else:
        print(f"Unknown group: {group}")
        sys.exit(1)