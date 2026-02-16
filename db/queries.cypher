// ============ 1. ПРОСТЫЕ ЗАПРОСЫ (фильтрация по свойствам и связям) ============

// 1.1 Найти все статьи автора с id=1
MATCH (a:Author {id: 1})-[:AUTHORED]->(art:Article)
RETURN a.name, art.title, art.created_at
ORDER BY art.created_at DESC;

// 1.2 Найти все статьи в категории "Технологии" (id=1)
MATCH (art:Article)-[:IN_CATEGORY]->(c:Category {name: 'Технологии'})
RETURN art.title, c.name, art.views
ORDER BY art.views DESC
LIMIT 5;

// 1.3 Найти всех пользователей с premium подпиской
MATCH (u:User {subscription_type: 'premium', is_active: true})
RETURN u.name, u.email, u.age;

// 1.4 Найти все статьи из источника TechCrunch
MATCH (art:Article)-[:FROM_SOURCE]->(s:Source {name: 'TechCrunch'})
RETURN art.title, art.created_at, art.views;

// 1.5 Найти все теги статьи с id=1
MATCH (art:Article {id: 1})-[:TAGGED_WITH]->(t:Tag)
RETURN art.title, collect(t.name) as tags, count(t) as tags_count;

// 1.6 Найти все статьи, которые лайкнул пользователь с id=1
MATCH (u:User {id: 1})-[r:LIKED]->(art:Article)
RETURN u.name, art.title, r.timestamp;

// ============ 2. ЦЕПОЧКИ СВЯЗЕЙ И ПЕРЕМЕННАЯ ДЛИНА ПУТИ ============

// 2.1 Найти пользователей, которые читают те же статьи, что и пользователь с id=1 (длина 2)
MATCH (u1:User {id: 1})-[:READ]->(art:Article)<-[:READ]-(u2:User)
WHERE u1.id <> u2.id
RETURN u1.name, u2.name, collect(art.title) as common_articles, count(art) as common_count
ORDER BY common_count DESC;

// 2.2 Найти косвенные связи: пользователи, связанные через общие категории (длина 2-4)
MATCH path = (u1:User {id: 1})-[:SUBSCRIBED_TO*2..4]-(u2:User)
WHERE u1.id <> u2.id AND ALL(n IN nodes(path) WHERE n:User OR n:Category)
RETURN u1.name, u2.name, length(path) as path_length, 
       [n IN nodes(path) WHERE n:Category | n.name] as via_categories
LIMIT 10;

// 2.3 Найти все пути между авторами через статьи и теги
MATCH path = (a1:Author {id: 1})-[:AUTHORED]->(:Article)-[:TAGGED_WITH]->(:Tag)<-[:TAGGED_WITH]-(:Article)<-[:AUTHORED]-(a2:Author)
WHERE a1.id <> a2.id
RETURN a1.name, a2.name, length(path) as path_length,
       [n IN nodes(path) WHERE n:Article | n.title] as articles
LIMIT 5;

// 2.4 Найти рекомендации для пользователя: статьи, которые читали пользователи со схожими интересами
MATCH (u:User {id: 1})-[:SUBSCRIBED_TO]->(c:Category)<-[:SUBSCRIBED_TO]-(similar:User)
MATCH (similar)-[:READ]->(rec:Article)
WHERE NOT EXISTS((u)-[:READ]->(rec))
RETURN rec.title, rec.views, collect(DISTINCT similar.name) as recommended_by
ORDER BY rec.views DESC
LIMIT 10;

// 2.5 Найти связанные статьи через общие теги (глубина 2)
MATCH (art1:Article {id: 1})-[:TAGGED_WITH]->(t:Tag)<-[:TAGGED_WITH]-(art2:Article)
WHERE art1.id <> art2.id
RETURN art1.title, art2.title, collect(t.name) as common_tags, count(t) as tags_count
ORDER BY tags_count DESC;

// ============ 3. АГРЕГАЦИОННЫЕ ЗАПРОСЫ ============

// 3.1 Топ-5 авторов по количеству статей и сумме просмотров
MATCH (a:Author)-[:AUTHORED]->(art:Article)
RETURN a.name, 
       count(art) as articles_count, 
       sum(art.views) as total_views,
       avg(art.views) as avg_views,
       collect(art.title)[..3] as top_articles
ORDER BY total_views DESC
LIMIT 5;

// 3.2 Статистика по категориям
MATCH (c:Category)<-[:IN_CATEGORY]-(art:Article)
OPTIONAL MATCH (u:User)-[:SUBSCRIBED_TO]->(c)
RETURN c.name, 
       count(DISTINCT art) as articles_count,
       sum(art.views) as total_views,
       count(DISTINCT u) as subscribers,
       avg(art.comments_count) as avg_comments
ORDER BY total_views DESC;

// 3.3 Активность пользователей
MATCH (u:User)
OPTIONAL MATCH (u)-[r:READ]->(art:Article)
WITH u, count(r) as reads, sum(art.views) as viewed_articles_total
OPTIONAL MATCH (u)-[:COMMENTED]->()
RETURN u.name, 
       reads,
       viewed_articles_total,
       count(u) as comments_count
ORDER BY reads DESC
LIMIT 5;

// 3.4 Топ-10 самых популярных статей
MATCH (art:Article)
RETURN art.title, 
       art.views, 
       art.likes, 
       art.comments_count,
       [(art)<-[:AUTHORED]-(a:Author) | a.name][0] as author
ORDER BY art.views DESC
LIMIT 10;

// 3.5 Статистика по тегам
MATCH (t:Tag)<-[:TAGGED_WITH]-(art:Article)
RETURN t.name, 
       count(art) as usage_count,
       sum(art.views) as total_views,
       avg(art.views) as avg_views,
       collect(art.title)[..3] as example_articles
ORDER BY usage_count DESC;

// ============ 4. ПОИСК ОБЩИХ СОСЕДЕЙ ============

// 4.1 Пользователи с общими категориями подписок
MATCH (u1:User)-[:SUBSCRIBED_TO]->(c:Category)<-[:SUBSCRIBED_TO]-(u2:User)
WHERE id(u1) < id(u2)
RETURN u1.name, u2.name, 
       count(c) as common_categories,
       collect(c.name) as categories
ORDER BY common_categories DESC;

// 4.2 Общие теги между статьями
MATCH (art1:Article)-[:TAGGED_WITH]->(t:Tag)<-[:TAGGED_WITH]-(art2:Article)
WHERE art1.id < art2.id
RETURN art1.title, art2.title, 
       count(t) as common_tags,
       collect(t.name) as tags,
       (art1.views + art2.views) as total_views
ORDER BY common_tags DESC
LIMIT 10;

// 4.3 Топ пользователей по количеству общих прочитанных статей с пользователем id=1
MATCH (u1:User {id: 1})-[:READ]->(art:Article)<-[:READ]-(u2:User)
WHERE u1.id <> u2.id
RETURN u1.name as user, 
       u2.name as similar_user, 
       count(art) as common_reads,
       collect(art.title) as articles
ORDER BY common_reads DESC
LIMIT 5;

// ============ 5. СЛОЖНЫЙ КОМБИНИРОВАННЫЙ ЗАПРОС ============

// 5.1 Найти рекомендации для автора: какие темы популярны среди читателей его статей
MATCH (a:Author {id: 1})-[:AUTHORED]->(art:Article)
MATCH (art)<-[:READ]-(u:User)
MATCH (u)-[:READ]->(other_art:Article)
WHERE other_art.id <> art.id
OPTIONAL MATCH (other_art)-[:TAGGED_WITH]->(t:Tag)
WITH a, t, count(DISTINCT u) as interested_users, count(DISTINCT other_art) as recommended_articles
WHERE t IS NOT NULL
RETURN a.name, 
       t.name as recommended_topic,
       interested_users,
       recommended_articles
ORDER BY interested_users DESC
LIMIT 5;

// 5.2 Найти "инфлюенсеров" - пользователей, чьи лайки наиболее совпадают с предпочтениями других
MATCH (u:User)-[:LIKED]->(art:Article)<-[:LIKED]-(other:User)
WHERE u.id <> other.id
WITH u, count(DISTINCT other) as influenced_users, collect(DISTINCT art.title)[..5] as liked_articles
WHERE influenced_users > 1
RETURN u.name, influenced_users, size(liked_articles) as unique_likes, liked_articles
ORDER BY influenced_users DESC
LIMIT 5;

// 5.3 Анализ "воронки" категорий: какие категории ведут к каким через статьи
MATCH path = (c1:Category)<-[:IN_CATEGORY]-(art:Article)-[:TAGGED_WITH]->(t:Tag)<-[:TAGGED_WITH]-(:Article)-[:IN_CATEGORY]->(c2:Category)
WHERE c1.id <> c2.id
RETURN c1.name as from_category, 
       c2.name as to_category,
       count(DISTINCT art) as bridging_articles,
       collect(DISTINCT t.name) as common_themes
ORDER BY bridging_articles DESC
LIMIT 10;