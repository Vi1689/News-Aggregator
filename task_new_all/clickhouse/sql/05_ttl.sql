-- ============================================================
-- TTL ПОЛИТИКА ХРАНЕНИЯ ДАННЫХ
-- Логика:
-- NewsViewed   — хранить 3 месяца (много данных, низкая ценность)
-- NewsLiked    — хранить 6 месяцев (полезно для аналитики)
-- NewsPublished — хранить 24 месяца (архив контента)
-- Остальные    — хранить 12 месяцев
-- ============================================================

ALTER TABLE news.news_events
    MODIFY TTL
        multiIf(
            event_type = 'NewsViewed',   event_time + INTERVAL 3 MONTH,
            event_type = 'NewsLiked',    event_time + INTERVAL 6 MONTH,
            event_type = 'NewsPublished', event_time + INTERVAL 24 MONTH,
            event_time + INTERVAL 12 MONTH
        );