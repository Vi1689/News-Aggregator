#!/bin/bash
set -e

BACKUP_ROOT="/backups"
DATE=$(date +%Y%m%d_%H%M%S)
LOG_FILE="$BACKUP_ROOT/logs/backup_${DATE}.log"

export PGPASSWORD="news_pass"

log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1" | tee -a $LOG_FILE
}

DAILY_DIR="$BACKUP_ROOT/daily/$DATE"
mkdir -p $DAILY_DIR

# ==================== БАЗА ДАННЫХ ====================
log "=== Database Backup ==="

# 1. Полный дамп БД в custom формате (основной бэкап)
log "Creating full database dump..."
pg_dump -h db-master -U news_user -d news_db \
    --format=custom \
    --verbose \
    --file="$DAILY_DIR/news_db_full_${DATE}.dump" 2>> $LOG_FILE

# 2. Дамп только схемы (читаемый SQL) для документации
log "Creating schema dump..."
pg_dump -h db-master -U news_user -d news_db \
    --schema-only \
    --format=plain \
    --file="$DAILY_DIR/news_db_schema_${DATE}.sql" 2>> $LOG_FILE

# 3. Глобальные объекты (пользователи, роли)
log "Creating globals dump..."
pg_dumpall -h db-master -U news_user \
    --globals-only \
    --file="$DAILY_DIR/news_db_globals_${DATE}.sql" 2>> $LOG_FILE

# ==================== НАСТРОЙКИ БД ====================
log "=== Database Configuration Backup ==="

# 4. Конфигурационные файлы PostgreSQL
if [ "$DOCKER_AVAILABLE" = "true" ]; then
    log "Backing up PostgreSQL configuration from master..."
    
    # Основные конфиги через прямой путь
    if docker exec db-master test -f /var/lib/postgresql/data/postgresql.conf 2>/dev/null; then
        docker exec db-master cat /var/lib/postgresql/data/postgresql.conf > $DAILY_DIR/postgresql.conf 2>/dev/null
        log "✓ postgresql.conf copied"
    else
        log "⚠ postgresql.conf not found in standard location"
    fi

    if docker exec db-master test -f /var/lib/postgresql/data/postgresql.auto.conf 2>/dev/null; then
        docker exec db-master cat /var/lib/postgresql/data/postgresql.auto.conf > $DAILY_DIR/postgresql.auto.conf 2>/dev/null
        log "✓ postgresql.auto.conf copied"
    else
        log "⚠ postgresql.auto.conf not found in standard location"
    fi

    # Резервное копирование через find
    if [ ! -s "$DAILY_DIR/postgresql.conf" ]; then
        log "Trying alternative method to find postgresql.conf..."
        docker exec db-master find /var/lib/postgresql/data -name "postgresql.conf" -exec cat {} \; > $DAILY_DIR/postgresql.conf 2>/dev/null && log "✓ postgresql.conf found via search" || log "❌ Could not copy postgresql.conf"
    fi

    # Копирование pg_hba.conf
    if docker exec db-master test -f /etc/postgresql/pg_hba.conf 2>/dev/null; then
        docker exec db-master cat /etc/postgresql/pg_hba.conf > $DAILY_DIR/pg_hba.conf 2>/dev/null
        log "✓ pg_hba.conf copied"
    else
        log "⚠ pg_hba.conf not found"
    fi
else
    log "Skipping configuration file backup - Docker not available"
fi

# 5. Настройки репликации и WAL (всегда доступно через psql)
log "Backing up replication configuration..."
psql -h db-master -U news_user -d news_db \
    -c "SELECT name, setting, unit, context FROM pg_settings 
        WHERE name LIKE '%replication%' 
           OR name LIKE '%wal%' 
           OR name LIKE '%archive%'
           OR name LIKE '%max_connections%'
           OR name LIKE '%shared_buffers%'
        ORDER BY name;" > $DAILY_DIR/db_settings.txt 2>> $LOG_FILE

# 6. Статус репликации
log "Backing up replication status..."
psql -h db-master -U news_user -d news_db \
    -c "SELECT now() as check_time, * FROM pg_stat_replication;" > $DAILY_DIR/replication_status.txt 2>> $LOG_FILE

# 7. Информация о таблицах и размерах
log "Backing up database metadata..."
psql -h db-master -U news_user -d news_db \
    -c "SELECT schemaname, tablename, tableowner, tablespace 
        FROM pg_tables 
        WHERE schemaname NOT LIKE 'pg_%' AND schemaname != 'information_schema'
        ORDER BY schemaname, tablename;" > $DAILY_DIR/tables_list.txt 2>> $LOG_FILE

psql -h db-master -U news_user -d news_db \
    -c "SELECT datname, 
               pg_size_pretty(pg_database_size(datname)) as size,
               pg_database_size(datname) as size_bytes
        FROM pg_database 
        WHERE datname = 'news_db';" > $DAILY_DIR/database_size.txt 2>> $LOG_FILE

# 8. Дополнительная информация о БД
log "Backing up additional database info..."
psql -h db-master -U news_user -d news_db \
    -c "SELECT version();" > $DAILY_DIR/postgres_version.txt 2>> $LOG_FILE

psql -h db-master -U news_user -d news_db \
    -c "SELECT now() as backup_time;" > $DAILY_DIR/backup_timestamp.txt 2>> $LOG_FILE

# 9. Проверка реплики
log "Checking replica status..."
if pg_isready -h db-replica -U news_user -d news_db -t 5 > /dev/null 2>&1; then
    psql -h db-replica -U news_user -d news_db \
        -c "SELECT now() as current_time, 
                   pg_is_in_recovery() as is_replica, 
                   pg_last_wal_receive_lsn() as receive_lsn, 
                   pg_last_wal_replay_lsn() as replay_lsn,
                   pg_last_xact_replay_timestamp() as replay_time,
                   now() - pg_last_xact_replay_timestamp() AS replication_lag,
                   pg_wal_lsn_diff(pg_last_wal_receive_lsn(), pg_last_wal_replay_lsn()) as lag_bytes;" > $DAILY_DIR/replica_status.txt 2>> $LOG_FILE
    log "✓ Replica status saved"
else
    echo "Replica not available at $(date)" > $DAILY_DIR/replica_status.txt
    log "⚠ Replica not available for status check"
fi

# ==================== ФИНАЛИЗАЦИЯ ====================
log "=== Finalizing Backup ==="

# 10. Создание checksum для проверки целостности
log "Creating checksums..."
cd $DAILY_DIR
if command -v md5sum > /dev/null 2>&1; then
    find . -type f -not -name "checksums.md5" -exec md5sum {} \; > checksums.md5 2>/dev/null && log "✓ Checksums created" || log "⚠ Could not create checksums"
else
    log "⚠ md5sum not available - skipping checksums"
fi

# 11. Создание README с информацией о бэкапе
log "Creating README..."
cat > $DAILY_DIR/README.txt << EOF
News Aggregator Database Backup
Created: $(date)
Backup ID: ${DATE}

Contains database-related files only:
- Full database dump (custom format): news_db_full_${DATE}.dump
- Schema only (SQL format): news_db_schema_${DATE}.sql  
- Global objects (users, roles): news_db_globals_${DATE}.sql
- PostgreSQL configuration files
- Replication settings and status
- Database metadata and sizes

Restore instructions:
1. Restore globals: psql -h db-master -U news_user -f news_db_globals_${DATE}.sql
2. Restore database: pg_restore -h db-master -U news_user -d news_db news_db_full_${DATE}.dump
3. Verify: psql -h db-master -U news_user -d news_db -c "SELECT COUNT(*) FROM pg_tables;"

File list:
$(find . -type f -not -name "README.txt" | sort)
EOF

# 12. Архивирование
log "Creating archive..."
if tar -czf $BACKUP_ROOT/data/daily_${DATE}.tar.gz -C $DAILY_DIR . 2>> $LOG_FILE; then
    log "✓ Archive created: daily_${DATE}.tar.gz"
else
    log "❌ Failed to create archive"
    exit 1
fi

# 13. Проверка размера архива
ARCHIVE_SIZE=$(du -h $BACKUP_ROOT/data/daily_${DATE}.tar.gz | cut -f1)
log "Archive size: $ARCHIVE_SIZE"

# Очистка временных файлов
rm -rf $DAILY_DIR
log "✓ Temporary files cleaned"

log "DATABASE-ONLY backup completed: $BACKUP_ROOT/data/daily_${DATE}.tar.gz"

# 14. Ротация бэкапов (храним 7 дней)
log "Running backup rotation..."
find $BACKUP_ROOT/data -name "daily_*.tar.gz" -mtime +7 -delete && log "✓ Old backups cleaned (7+ days)"
find $BACKUP_ROOT/logs -name "backup_*.log" -mtime +30 -delete && log "✓ Old logs cleaned (30+ days)"

# 15. Финальный статус
BACKUP_COUNT=$(find $BACKUP_ROOT/data -name "daily_*.tar.gz" | wc -l)
log "Backup process finished successfully. Total backups: $BACKUP_COUNT"