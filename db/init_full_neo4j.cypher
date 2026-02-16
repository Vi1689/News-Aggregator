// ===========================================
// ПОЛНАЯ ИНИЦИАЛИЗАЦИЯ NEO4J ДЛЯ NEWS-AGGREGATOR
// Создание пользователей, ограничений, индексов и тестовых данных
// ===========================================

// === 1. СОЗДАНИЕ ПОЛЬЗОВАТЕЛЕЙ И РОЛЕЙ ===
CREATE USER reader IF NOT EXISTS SET PASSWORD 'readerpass' CHANGE NOT REQUIRED;
GRANT ROLE reader TO reader;

CREATE USER publisher IF NOT EXISTS SET PASSWORD 'publisherpass' CHANGE NOT REQUIRED;
GRANT ROLE publisher TO publisher;

// === 2. СОЗДАНИЕ ОГРАНИЧЕНИЙ (CONSTRAINTS) ===
CREATE CONSTRAINT user_id_unique IF NOT EXISTS FOR (u:User) REQUIRE u.id IS UNIQUE;
CREATE CONSTRAINT user_email_unique IF NOT EXISTS FOR (u:User) REQUIRE u.email IS UNIQUE;
CREATE CONSTRAINT author_id_unique IF NOT EXISTS FOR (a:Author) REQUIRE a.id IS UNIQUE;
CREATE CONSTRAINT editor_id_unique IF NOT EXISTS FOR (e:Editor) REQUIRE e.id IS UNIQUE;
CREATE CONSTRAINT category_id_unique IF NOT EXISTS FOR (c:Category) REQUIRE c.id IS UNIQUE;
CREATE CONSTRAINT tag_id_unique IF NOT EXISTS FOR (t:Tag) REQUIRE t.id IS UNIQUE;
CREATE CONSTRAINT source_id_unique IF NOT EXISTS FOR (s:Source) REQUIRE s.id IS UNIQUE;
CREATE CONSTRAINT article_id_unique IF NOT EXISTS FOR (art:Article) REQUIRE art.id IS UNIQUE;

// === 3. СОЗДАНИЕ ИНДЕКСОВ ===
CREATE INDEX user_name_index IF NOT EXISTS FOR (u:User) ON (u.name);
CREATE INDEX user_email_index IF NOT EXISTS FOR (u:User) ON (u.email);
CREATE INDEX user_subscription_index IF NOT EXISTS FOR (u:User) ON (u.subscription_type);
CREATE INDEX article_title_index IF NOT EXISTS FOR (art:Article) ON (art.title);
CREATE INDEX article_created_at_index IF NOT EXISTS FOR (art:Article) ON (art.created_at);
CREATE INDEX article_views_index IF NOT EXISTS FOR (art:Article) ON (art.views);
CREATE INDEX article_source_index IF NOT EXISTS FOR (art:Article) ON (art.source);
CREATE INDEX author_name_index IF NOT EXISTS FOR (a:Author) ON (a.name);
CREATE INDEX category_name_index IF NOT EXISTS FOR (c:Category) ON (c.name);
CREATE INDEX tag_name_index IF NOT EXISTS FOR (t:Tag) ON (t.name);
CREATE INDEX source_name_index IF NOT EXISTS FOR (s:Source) ON (s.name);

// Полнотекстовые индексы для поиска
CREATE FULLTEXT INDEX article_content_fulltext IF NOT EXISTS 
FOR (n:Article) ON EACH [n.title, n.content];

CREATE FULLTEXT INDEX user_search_fulltext IF NOT EXISTS 
FOR (n:User) ON EACH [n.name, n.email];

// === 4. ОЧИСТКА ТЕСТОВЫХ ДАННЫХ (если нужно пересоздать) ===
// MATCH (n) DETACH DELETE n;

// === 5. СОЗДАНИЕ ТЕСТОВЫХ ДАННЫХ ===
// ============ СОЗДАНИЕ УЗЛОВ ============

// Создаем пользователей (10 пользователей)
FOREACH (id IN range(1, 10) |
  MERGE (u:User {id: id})
  ON CREATE SET 
    u.name = CASE id
      WHEN 1 THEN 'Алексей Петров'
      WHEN 2 THEN 'Мария Иванова'
      WHEN 3 THEN 'Дмитрий Соколов'
      WHEN 4 THEN 'Елена Козлова'
      WHEN 5 THEN 'Анна Смирнова'
      WHEN 6 THEN 'Павел Морозов'
      WHEN 7 THEN 'Ольга Новикова'
      WHEN 8 THEN 'Игорь Волков'
      WHEN 9 THEN 'Татьяна Павлова'
      WHEN 10 THEN 'Сергей Федоров'
    END,
    u.email = CASE id
      WHEN 1 THEN 'alexey@example.com'
      WHEN 2 THEN 'maria@example.com'
      WHEN 3 THEN 'dmitry@example.com'
      WHEN 4 THEN 'elena@example.com'
      WHEN 5 THEN 'anna@example.com'
      WHEN 6 THEN 'pavel@example.com'
      WHEN 7 THEN 'olga@example.com'
      WHEN 8 THEN 'igor@example.com'
      WHEN 9 THEN 'tatyana@example.com'
      WHEN 10 THEN 'sergey@example.com'
    END,
    u.age = 20 + id * 2,
    u.registered_at = datetime() - duration({days: id * 30}),
    u.is_active = id % 3 <> 0,
    u.subscription_type = CASE id % 3
      WHEN 0 THEN 'premium'
      WHEN 1 THEN 'basic'
      ELSE 'free'
    END
);

// Создаем авторов (5 авторов - некоторые из них также пользователи)
FOREACH (id IN range(1, 5) |
  MERGE (a:Author {id: id})
  ON CREATE SET 
    a.name = CASE id
      WHEN 1 THEN 'Иван Петров'
      WHEN 2 THEN 'Светлана Иванова'
      WHEN 3 THEN 'Михаил Сидоров'
      WHEN 4 THEN 'Наталья Козлова'
      WHEN 5 THEN 'Андрей Смирнов'
    END,
    a.bio = CASE id
      WHEN 1 THEN 'Технический журналист, специализируется на AI'
      WHEN 2 THEN 'Обозреватель новостей IT'
      WHEN 3 THEN 'Аналитик рынка технологий'
      WHEN 4 THEN 'Редактор раздела Наука'
      WHEN 5 THEN 'Колумнист и блогер'
    END,
    a.since = date('2020-01-01') + duration({months: id * 3}),
    a.rating = rand() * 5,
    a.articles_count = id * 10
);

// Создаем редакторов (3 редактора)
FOREACH (id IN range(1, 3) |
  MERGE (e:Editor {id: id})
  ON CREATE SET 
    e.name = CASE id
      WHEN 1 THEN 'Главный редактор'
      WHEN 2 THEN 'Научный редактор'
      WHEN 3 THEN 'Технический редактор'
    END,
    e.department = CASE id
      WHEN 1 THEN 'general'
      WHEN 2 THEN 'science'
      WHEN 3 THEN 'tech'
    END,
    e.experience_years = id + 5
);

// Создаем категории (6 категорий)
FOREACH (id IN range(1, 6) |
  MERGE (c:Category {id: id})
  ON CREATE SET 
    c.name = CASE id
      WHEN 1 THEN 'Технологии'
      WHEN 2 THEN 'Наука'
      WHEN 3 THEN 'Бизнес'
      WHEN 4 THEN 'Искусственный интеллект'
      WHEN 5 THEN 'Программирование'
      WHEN 6 THEN 'Кибербезопасность'
    END,
    c.description = 'Категория: ' + c.name,
    c.posts_count = id * 15,
    c.color = CASE id % 3
      WHEN 0 THEN 'red'
      WHEN 1 THEN 'blue'
      ELSE 'green'
    END
);

// Создаем теги (8 тегов)
FOREACH (id IN range(1, 8) |
  MERGE (t:Tag {id: id})
  ON CREATE SET 
    t.name = CASE id
      WHEN 1 THEN 'нейросети'
      WHEN 2 THEN 'блокчейн'
      WHEN 3 THEN 'стартапы'
      WHEN 4 THEN 'исследования'
      WHEN 5 THEN 'python'
      WHEN 6 THEN 'bigdata'
      WHEN 7 THEN 'cloud'
      WHEN 8 THEN 'devops'
    END,
    t.popularity = rand() * 100
);

// Создаем источники (4 источника)
FOREACH (id IN range(1, 4) |
  MERGE (s:Source {id: id})
  ON CREATE SET 
    s.name = CASE id
      WHEN 1 THEN 'TechCrunch'
      WHEN 2 THEN 'Habr'
      WHEN 3 THEN 'Medium'
      WHEN 4 THEN 'BBC News'
    END,
    s.country = CASE id
      WHEN 1 THEN 'USA'
      WHEN 2 THEN 'Russia'
      WHEN 3 THEN 'Global'
      WHEN 4 THEN 'UK'
    END,
    s.trust_level = CASE id
      WHEN 1 THEN 80
      WHEN 2 THEN 70
      WHEN 3 THEN 85
      WHEN 4 THEN 90
    END
);

// Создаем статьи (20 статей)
FOREACH (id IN range(1, 20) |
  MERGE (art:Article {id: id})
  ON CREATE SET 
    art.title = CASE id % 10
      WHEN 0 THEN 'Будущее искусственного интеллекта'
      WHEN 1 THEN 'Как начать программировать на Python'
      WHEN 2 THEN 'Тренды в кибербезопасности 2026'
      WHEN 3 THEN 'Облачные технологии для бизнеса'
      WHEN 4 THEN 'Нейросети в медицине'
      WHEN 5 THEN 'Блокчейн вне криптовалют'
      WHEN 6 THEN '10 стартапов, за которыми следить'
      WHEN 7 THEN 'DevOps практики в 2026'
      WHEN 8 THEN 'Big Data аналитика'
      WHEN 9 THEN 'Квантовые компьютеры'
    END + ' #' + id,
    art.content = 'Содержание статьи ' + art.title + '...',
    art.created_at = datetime() - duration({days: id * 3}),
    art.views = toInteger(rand() * 10000),
    art.likes = toInteger(rand() * 1000),
    art.comments_count = toInteger(rand() * 100),
    art.is_published = id % 2 = 0,
    art.reading_time_min = 5 + id % 10
);

// ============ СОЗДАНИЕ СВЯЗЕЙ (100+ связей) ============

// Связи автор-статья (AUTHORED)
MATCH (a:Author), (art:Article)
WHERE a.id = art.id % 5 + 1
MERGE (a)-[:AUTHORED {timestamp: art.created_at}]->(art);

// Связи редактор-статья (EDITED)
MATCH (e:Editor), (art:Article)
WHERE e.id = art.id % 3 + 1 AND art.id % 2 = 0
MERGE (e)-[:EDITED {
  timestamp: art.created_at + duration({hours: 2}),
  changes_made: toInteger(rand() * 50)
}]->(art);

// Связи статья-категория (IN_CATEGORY)
MATCH (art:Article), (c:Category)
WHERE c.id = art.id % 6 + 1
MERGE (art)-[:IN_CATEGORY {relevance: rand() * 100}]->(c);

// Связи статья-тег (TAGGED_WITH)
MATCH (art:Article), (t:Tag)
WHERE t.id = art.id % 8 + 1 OR t.id = (art.id + 3) % 8 + 1
MERGE (art)-[:TAGGED_WITH {weight: rand()}]->(t);

// Связи статья-источник (FROM_SOURCE)
MATCH (art:Article), (s:Source)
WHERE s.id = art.id % 4 + 1
MERGE (art)-[:FROM_SOURCE {published_at: art.created_at}]->(s);

// Связи пользователь-статья (READ, LIKED, COMMENTED, FAVORITED)
MATCH (u:User), (art:Article)
WHERE u.id % 3 = art.id % 3
FOREACH (_ IN range(1, 3) |
  MERGE (u)-[:READ {
    timestamp: art.created_at + duration({hours: u.id}),
    read_time_seconds: toInteger(rand() * 600)
  }]->(art)
);

MATCH (u:User), (art:Article)
WHERE u.id % 4 = art.id % 4
MERGE (u)-[:LIKED {timestamp: datetime()}]->(art);

MATCH (u:User), (art:Article)
WHERE u.id % 5 = art.id % 5
MERGE (u)-[:COMMENTED {
  timestamp: datetime(),
  comment: 'Отличная статья! ' + u.name
}]->(art);

MATCH (u:User), (art:Article)
WHERE u.id % 2 = art.id % 2
MERGE (u)-[:FAVORITED {added_at: datetime()}]->(art);

// Связи подписки пользователь-категория (SUBSCRIBED_TO)
MATCH (u:User), (c:Category)
WHERE u.id % 3 = c.id % 3
MERGE (u)-[:SUBSCRIBED_TO {since: datetime() - duration({days: u.id * 10})}]->(c);

// Связи пользователь-пользователь (FOLLOWS)
MATCH (u1:User), (u2:User)
WHERE u1.id <> u2.id AND u1.id % 3 = u2.id % 3
MERGE (u1)-[:FOLLOWS {since: datetime() - duration({days: u1.id * 5})}]->(u2);

// Связи автор-категория (SPECIALIZES_IN)
MATCH (a:Author), (c:Category)
WHERE a.id % 3 = c.id % 3
MERGE (a)-[:SPECIALIZES_IN {level: rand() * 10}]->(c);

// === 6. ПРОВЕРКА СОЗДАННЫХ ДАННЫХ ===
RETURN '=== ИНИЦИАЛИЗАЦИЯ ЗАВЕРШЕНА ===' as message,
       'Пользователи: ' + toString(count {MATCH (u:User)}) + 
       ', Авторы: ' + toString(count {MATCH (a:Author)}) +
       ', Статьи: ' + toString(count {MATCH (art:Article)}) +
       ', Связи: ' + toString(count {MATCH ()-[r]->()}) as stats;