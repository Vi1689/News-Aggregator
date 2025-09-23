import psycopg2
from faker import Faker
import random

fake = Faker()

conn = psycopg2.connect(
    dbname="news_db",
    user="news_user",
    password="news_pass",
    host="db",
    port="5432"
)   
cur = conn.cursor()

# 1. Источники
print("Inserting sources...")
sources = [(fake.domain_name(),) for _ in range(800)]
cur.executemany("INSERT INTO sources (name) VALUES (%s)", sources)

# 2. Авторы
print("Inserting authors...")
authors = [(fake.name(),) for _ in range(80000)]
cur.executemany("INSERT INTO authors (name) VALUES (%s)", authors)

# 3. Категории
print("Inserting categories...")
categories = [("Политика",), ("Экономика",), ("Технологии",), ("Спорт",), ("Культура",), ("Наука",)]
cur.executemany("INSERT INTO categories (name) VALUES (%s)", categories)

# 4. Теги
print("Inserting tags...")
tags = [(fake.word(),) for _ in range(10000)]
cur.executemany("INSERT INTO tags (name) VALUES (%s)", tags)

conn.commit()

# Получим id-шники
cur.execute("SELECT id FROM sources")
sources_ids = [r[0] for r in cur.fetchall()]

cur.execute("SELECT id FROM authors")
authors_ids = [r[0] for r in cur.fetchall()]

cur.execute("SELECT id FROM categories")
categories_ids = [r[0] for r in cur.fetchall()]

cur.execute("SELECT id FROM tags")
tags_ids = [r[0] for r in cur.fetchall()]

# 5. Новости (3.2 млн)
print("Inserting news...")
batch_size = 10000
total_news = 3200000

for i in range(0, total_news, batch_size):
    batch = []
    for _ in range(batch_size):
        batch.append((
            fake.sentence(nb_words=8),                # title
            fake.text(max_nb_chars=500),              # content
            fake.date_time_this_decade(),             # published_at
            random.choice(authors_ids),               # author_id
            random.choice(sources_ids),               # source_id
            random.choice(categories_ids)             # category_id
        ))
    cur.executemany("""
        INSERT INTO news (title, content, published_at, author_id, source_id, category_id)
        VALUES (%s, %s, %s, %s, %s, %s)
    """, batch)
    conn.commit()
    print(f"Inserted {i + batch_size}/{total_news}")

# 6. news_tags (связи)
print("Inserting news_tags...")
for i in range(0, total_news, batch_size):
    batch = []
    batch_tags = []
    for _ in range(batch_size):
        title = fake.sentence(nb_words=8)
        content = fake.text(max_nb_chars=500)
        published_at = fake.date_time_this_decade()
        author_id = random.choice(authors_ids)
        source_id = random.choice(sources_ids)
        category_id = random.choice(categories_ids)

        batch.append((title, content, published_at, author_id, source_id, category_id))

    cur.executemany("""
        INSERT INTO news (title, content, published_at, author_id, source_id, category_id)
        VALUES (%s, %s, %s, %s, %s, %s) RETURNING id
    """, batch)

    # Получаем id вставленных новостей
    inserted_ids = [r[0] for r in cur.fetchall()]

    # Генерируем теги для этих новостей
    batch_tags = []
    for news_id in inserted_ids:
        for _ in range(random.randint(2,5)):
            batch_tags.append((news_id, random.choice(tags_ids)))

    cur.executemany("INSERT INTO news_tags (news_id, tag_id) VALUES (%s, %s) ON CONFLICT DO NOTHING", batch_tags)
    conn.commit()
    print(f"Inserted {i + batch_size}/{total_news}")


batch = []
for news_id in news_ids:
    for _ in range(random.randint(2, 5)):
        batch.append((news_id, random.choice(tags_ids)))
    if len(batch) > 100000:
        cur.executemany("INSERT INTO news_tags (news_id, tag_id) VALUES (%s, %s) ON CONFLICT DO NOTHING", batch)
        conn.commit()
        batch = []

if batch:
    cur.executemany("INSERT INTO news_tags (news_id, tag_id) VALUES (%s, %s) ON CONFLICT DO NOTHING", batch)
    conn.commit()

print("✅ Data generation completed")
cur.close()
conn.close()
