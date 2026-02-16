// Создаем пользователей и роли
CREATE USER reader IF NOT EXISTS SET PASSWORD 'readerpass' CHANGE NOT REQUIRED;
GRANT ROLE reader TO reader;

CREATE USER publisher IF NOT EXISTS SET PASSWORD 'publisherpass' CHANGE NOT REQUIRED;
GRANT ROLE publisher TO publisher;

// Создаем ограничения
CREATE CONSTRAINT user_id_unique IF NOT EXISTS
FOR (u:User) REQUIRE u.id IS UNIQUE;

CREATE CONSTRAINT user_email_unique IF NOT EXISTS
FOR (u:User) REQUIRE u.email IS UNIQUE;

CREATE CONSTRAINT article_id_unique IF NOT EXISTS
FOR (a:Article) REQUIRE a.id IS UNIQUE;

// Создаем индексы
CREATE INDEX user_name_index IF NOT EXISTS
FOR (u:User) ON (u.name);

CREATE INDEX article_created_at_index IF NOT EXISTS
FOR (a:Article) ON (a.created_at);

CREATE INDEX article_source_index IF NOT EXISTS
FOR (a:Article) ON (a.source);

// Полнотекстовый индекс для поиска
CREATE FULLTEXT INDEX article_content_fulltext IF NOT EXISTS
FOR (n:Article) ON EACH [n.title, n.content];

// Создаем тестовые данные (опционально)
CREATE (u1:User {id: 1, name: 'Alice', email: 'alice@example.com', created_at: datetime()})
CREATE (u2:User {id: 2, name: 'Bob', email: 'bob@example.com', created_at: datetime()})
CREATE (a1:Article {id: 1, title: 'Neo4j Tutorial', content: 'Graph databases are awesome...', source: 'tech-blog', created_at: datetime()})
CREATE (a2:Article {id: 2, title: 'Docker Guide', content: 'Containerization with Docker...', source: 'dev-ops', created_at: datetime()})

// Создаем связи
CREATE (u1)-[:AUTHORED]->(a1)
CREATE (u1)-[:AUTHORED]->(a2)
CREATE (u2)-[:READ {timestamp: datetime()}]->(a1)
CREATE (u2)-[:LIKED {timestamp: datetime()}]->(a2)

RETURN 'Initialization complete' as message;