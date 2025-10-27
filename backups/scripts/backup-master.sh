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

log "Starting COMPLETE backup process..."

# ==================== БАЗА ДАННЫХ ====================
log "=== Database Backup ==="

# 1. Полный дамп БД в custom формате
log "Creating full database dump..."
pg_dump -h db-master -U news_user -d news_db \
    --format=custom \
    --verbose \
    --file="$DAILY_DIR/news_db_full_${DATE}.dump"

# 2. Дамп только схемы (читаемый SQL)
log "Creating schema dump..."
pg_dump -h db-master -U news_user -d news_db \
    --schema-only \
    --format=plain \
    --file="$DAILY_DIR/news_db_schema_${DATE}.sql"

# 3. Дамп только данных
log "Creating data-only dump..."
pg_dump -h db-master -U news_user -d news_db \
    --data-only \
    --format=custom \
    --file="$DAILY_DIR/news_db_data_${DATE}.dump"

# 4. Глобальные объекты (пользователи, роли)
log "Creating globals dump..."
pg_dumpall -h db-master -U news_user \
    --globals-only \
    --file="$DAILY_DIR/news_db_globals_${DATE}.sql"

# ==================== НАСТРОЙКИ БД ====================
log "=== Database Configuration Backup ==="

# 5. Конфигурационные файлы PostgreSQL
log "Backing up PostgreSQL configuration..."
docker cp db-master:/etc/postgresql/pg_hba.conf $DAILY_DIR/ > /dev/null 2>&1 && log "✓ pg_hba.conf copied" || log "⚠ Could not copy pg_hba.conf"

# Попробуем альтернативные пути для конфигов
docker exec db-master find /var/lib/postgresql/data -name "postgresql.conf" -exec cat {} \; > $DAILY_DIR/postgresql.conf 2>/dev/null && log "✓ postgresql.conf copied" || log "⚠ Could not copy postgresql.conf"

docker exec db-master find /var/lib/postgresql/data -name "postgresql.auto.conf" -exec cat {} \; > $DAILY_DIR/postgresql.auto.conf 2>/dev/null && log "✓ postgresql.auto.conf copied" || log "⚠ Could not copy postgresql.auto.conf"

# 6. Настройки репликации
log "Backing up replication configuration and status..."
psql -h db-master -U news_user -d news_db \
    -c "SELECT name, setting FROM pg_settings WHERE name LIKE '%replication%' OR name LIKE '%wal%' OR name LIKE '%archive%';" > $DAILY_DIR/replication_settings.txt

psql -h db-master -U news_user -d news_db \
    -c "SELECT * FROM pg_stat_replication;" > $DAILY_DIR/replication_status.txt

# 7. Информация о реплике
if pg_isready -h db-replica -U news_user -d news_db; then
    log "Backing up replica status..."
    psql -h db-replica -U news_user -d news_db \
        -c "SELECT now() as current_time, pg_is_in_recovery() as is_replica, pg_last_wal_receive_lsn() as receive_lsn, pg_last_wal_replay_lsn() as replay_lsn, pg_last_xact_replay_timestamp() as replay_time, now() - pg_last_xact_replay_timestamp() AS replication_lag;" > $DAILY_DIR/replica_status.txt
else
    echo "Replica not available" > $DAILY_DIR/replica_status.txt
    log "⚠ Replica not available for status check"
fi

# ==================== ИНИЦИАЛИЗАЦИОННЫЕ СКРИПТЫ ====================
log "=== Initialization Scripts Backup ==="

# 8. SQL скрипты инициализации - копируем с хоста
log "Backing up initialization scripts..."
cp /backups/../db/init.sql $DAILY_DIR/ > /dev/null 2>&1 && log "✓ init.sql copied" || log "⚠ Could not copy init.sql from host"

cp /backups/../db/tmp.sql $DAILY_DIR/ > /dev/null 2>&1 && log "✓ tmp.sql copied" || log "⚠ Could not copy tmp.sql from host"

cp /backups/../db/init-replica.sh $DAILY_DIR/ > /dev/null 2>&1 && log "✓ init-replica.sh copied" || log "⚠ Could not copy init-replica.sh from host"

# Также попробуем из контейнеров
docker cp db-master:/docker-entrypoint-initdb.d/init.sql $DAILY_DIR/init.sql.from_container > /dev/null 2>&1 && log "✓ init.sql from container" || log "⚠ Could not copy init.sql from container"

# ==================== DOCKER КОНФИГИ ====================
log "=== Docker Configuration Backup ==="

# 9. Docker композ файлы
log "Backing up Docker configurations..."
cp /backups/../docker-compose.yml $DAILY_DIR/ > /dev/null 2>&1 && log "✓ docker-compose.yml copied" || log "⚠ Could not copy docker-compose.yml"

# 10. Информация о системе
log "Backing up system information..."
docker version > $DAILY_DIR/docker_version.txt 2>/dev/null && log "✓ Docker version info saved" || log "⚠ Could not get Docker version"
docker compose version > $DAILY_DIR/docker_compose_version.txt 2>/dev/null && log "✓ Docker Compose version info saved" || log "⚠ Could not get Docker Compose version"

# ==================== ФИНАЛИЗАЦИЯ ====================
log "=== Finalizing Backup ==="

# 11. Создание checksum для проверки целостности
log "Creating checksums..."
cd $DAILY_DIR
find . -type f -not -name "checksums.md5" -exec md5sum {} \; > checksums.md5 2>/dev/null || log "⚠ Could not create checksums"

# 12. Создание README с информацией о бэкапе
cat > $DAILY_DIR/README.txt << EOF
News Aggregator Database Backup
Created: $(date)
Contains:
- Full database dump (custom format)
- Schema only (SQL format) 
- Data only (custom format)
- Global objects (users, roles)
- PostgreSQL configuration files
- Replication settings and status
- Initialization scripts
- Docker compose configuration

Restore instructions:
1. Extract archive: tar -xzf daily_${DATE}.tar.gz
2. Restore globals: psql -f news_db_globals_${DATE}.sql
3. Restore database: pg_restore -d news_db news_db_full_${DATE}.dump
EOF

# 13. Архивирование
log "Creating archive..."
tar -czf $BACKUP_ROOT/data/daily_${DATE}.tar.gz -C $DAILY_DIR .

# Очистка временных файлов
rm -rf $DAILY_DIR

log "COMPLETE backup completed: $BACKUP_ROOT/data/daily_${DATE}.tar.gz"

# Ротация бэкапов (храним 7 дней)
find $BACKUP_ROOT/data -name "daily_*.tar.gz" -mtime +7 -delete
find $BACKUP_ROOT/logs -name "backup_*.log" -mtime +30 -delete

log "Backup process finished successfully"