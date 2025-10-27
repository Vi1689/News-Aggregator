#!/bin/bash
set -e

# Ждем готовности мастера
until pg_isready -h db-master -U news_user -d news_db; do
  sleep 2
done

# Останавливаем PostgreSQL
pg_ctl -D /var/lib/postgresql/data stop

# Очищаем данные реплики
rm -rf /var/lib/postgresql/data/*

# Создаем базовый бэкап с мастера с настройкой репликации
pg_basebackup -h db-master -U news_user -D /var/lib/postgresql/data -P -R -X stream

# Запускаем PostgreSQL
pg_ctl -D /var/lib/postgresql/data start