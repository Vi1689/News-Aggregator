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

log "Starting backup process..."

# 1. Полный дамп БД
log "Creating full database dump..."
pg_dump -h db-master -U news_user -d news_db \
    --format=custom \
    --verbose \
    --file="$DAILY_DIR/news_db_full_${DATE}.dump"

# 2. Дамп схемы
log "Creating schema dump..."
pg_dump -h db-master -U news_user -d news_db \
    --schema-only \
    --format=plain \
    --file="$DAILY_DIR/news_db_schema_${DATE}.sql"

# 3. Глобальные объекты
log "Creating globals dump..."
pg_dumpall -h db-master -U news_user \
    --globals-only \
    --file="$DAILY_DIR/news_db_globals_${DATE}.sql"

# 4. Информация о репликации (если реплика готова)
log "Backing up replication info..."
if pg_isready -h db-replica -U news_user -d news_db; then
    psql -h db-master -U news_user -d news_db \
        -c "SELECT * FROM pg_stat_replication;" > $DAILY_DIR/replication_status.txt
else
    echo "Replica not ready" > $DAILY_DIR/replication_status.txt
fi

# 5. Архивирование
log "Creating archive..."
tar -czf $BACKUP_ROOT/data/daily_${DATE}.tar.gz -C $DAILY_DIR .

# Очистка
rm -rf $DAILY_DIR

log "Backup completed: $BACKUP_ROOT/data/daily_${DATE}.tar.gz"

# Ротация
find $BACKUP_ROOT/data -name "daily_*.tar.gz" -mtime +7 -delete
find $BACKUP_ROOT/logs -name "backup_*.log" -mtime +30 -delete

log "Backup process finished successfully"