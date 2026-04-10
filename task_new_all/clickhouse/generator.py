import uuid
import random
from datetime import datetime, timedelta
from clickhouse_driver import Client

client = Client(
    host='localhost',
    port=9000,
    user='admin',
    password='admin123',
    database='news'
)

# ============================================================
# ФИКСИРОВАННЫЕ СУЩНОСТИ (повторяются между событиями)
# ============================================================

# Новости (100 уникальных новостей)
NEWS_IDS = [uuid.uuid4() for _ in range(100)]
NEWS_TITLES = [
    f"Новость {i}: Квантовые компьютеры совершают прорыв" if i % 5 == 0
    else f"Статья {i}: Технологии будущего" if i % 3 == 0
    else f"Репортаж {i}: Главные события недели"
    for i in range(100)
]

# Пользователи (200 активных пользователей)
USERS = [f"user_{i:03d}" for i in range(200)] + ["demo_user"]

# Авторы (30 авторов)
AUTHORS = [(f"author_{i}", random.choice(['journalist', 'agency', 'blogger'])) for i in range(30)]

# Каналы/источники (15 каналов)
CHANNELS = [f"channel_{i}" for i in range(15)]

# Категории
CATEGORIES = ['technology', 'sports', 'politics', 'economy', 'science', 'health', 'culture']

# Теги (20 тегов)
TAGS = [
    'ai', 'blockchain', 'startup', 'innovation', 'breakthrough',
    'football', 'olympics', 'election', 'law', 'climate',
    'crypto', 'space', 'medicine', 'education', 'art',
    'cinema', 'music', 'travel', 'food', 'fashion'
]

# Типы событий и их веса
EVENT_TYPES = ['NewsPublished', 'NewsViewed', 'NewsLiked', 'NewsShared', 'CommentAdded']
EVENT_WEIGHTS = [0.05, 0.70, 0.15, 0.05, 0.05]  # 5% публикаций, 70% просмотров, 15% лайков и т.д.

# Платформы для шеринга
PLATFORMS = ['telegram', 'twitter', 'facebook', 'whatsapp']

def random_time():
    """Генерация времени с пиками активности (утро 8-10, вечер 18-21)"""
    now = datetime.now()
    base = now - timedelta(weeks=8)  # данные за 8 недель
    
    day_offset = random.randint(0, 55)
    dt = base + timedelta(days=day_offset)
    
    # 60% данных попадают в часы пик
    if random.random() < 0.6:
        peak = random.choice([(8, 10), (18, 21)])
        hour = random.randint(peak[0], peak[1])
    else:
        hour = random.randint(0, 23)
    
    return dt.replace(hour=hour, minute=random.randint(0, 59), second=random.randint(0, 59))

def generate_batch(n=100000):
    rows = []
    
    for _ in range(n):
        event_type = random.choices(EVENT_TYPES, weights=EVENT_WEIGHTS)[0]
        event_time = random_time()
        news_id = random.choice(NEWS_IDS)
        user_id = random.choice(USERS)
        author_name, author_type = random.choice(AUTHORS)
        
        row = {
            'event_id': uuid.uuid4(),
            'event_time': event_time,
            'event_type': event_type,
            'ingested_at': datetime.now(),
            
            # Идентификаторы
            'news_id': news_id,
            'user_id': user_id if event_type != 'NewsPublished' else '',
            'author_id': random.randint(1, 30) if event_type == 'NewsPublished' else 0,
            'channel_id': random.randint(1, 15),
            'comment_id': random.randint(1, 10000) if event_type == 'CommentAdded' else 0,
            
            # Денормализованные данные
            'title': random.choice(NEWS_TITLES) if event_type == 'NewsPublished' else '',
            'category': random.choice(CATEGORIES),
            'tags': random.sample(TAGS, random.randint(2, 5)),
            'author_name': author_name,
            'author_type': author_type,
            
            # Специфичные поля
            'read_duration_sec': random.randint(10, 600) if event_type == 'NewsViewed' else 0,
            'like_value': random.choice([1, 1, 1, -1]) if event_type == 'NewsLiked' else 0,  # 25% дизлайков
            'platform': random.choice(PLATFORMS) if event_type == 'NewsShared' else '',
            'shared_to_user_id': random.choice(USERS) if event_type == 'NewsShared' else '',
            'comment_text': f"Комментарий от {user_id}: интересная статья!" if event_type == 'CommentAdded' else '',
        }
        rows.append(row)
    
    return rows

print("Генерируем 100000 записей...")
rows = generate_batch(100000)

print("Загружаем в ClickHouse...")
client.execute('''
    INSERT INTO news.news_events (
        event_id, event_time, event_type, ingested_at,
        news_id, user_id, author_id, channel_id, comment_id,
        title, category, tags, author_name, author_type,
        read_duration_sec, like_value, platform, shared_to_user_id, comment_text
    ) VALUES
''', rows)

print("Проверяем...")
result = client.execute('SELECT event_type, count() FROM news.news_events GROUP BY event_type')
for row in result:
    print(f'  {row[0]}: {row[1]} записей')

print("Готово!")