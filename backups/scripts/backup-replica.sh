#!/bin/bash
set -e

BACKUP_ROOT="/backups"
DATE=$(date +%Y%m%d_%H%M%S)
LOG_FILE="$BACKUP_ROOT/logs/replica_monitor_${DATE}.log"

export PGPASSWORD="news_pass"

log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1" | tee -a $LOG_FILE
}

log "Starting replica monitoring..."

# Проверка доступности реплики
if ! pg_isready -h db-replica -U news_user -d news_db; then
    log "❌ ERROR: Replica is not accessible!"
    exit 1
fi

log "✅ Replica is accessible"

# 1. Основной статус репликации
log "Checking replication status..."
psql -h db-replica -U news_user -d news_db \
    -c "SELECT 
        now() as check_time,
        pg_is_in_recovery() as is_replica,
        pg_last_wal_receive_lsn() as receive_lsn,
        pg_last_wal_replay_lsn() as replay_lsn,
        pg_last_xact_replay_timestamp() as replay_time,
        now() - pg_last_xact_replay_timestamp() AS replication_lag,
        pg_wal_lsn_diff(pg_last_wal_receive_lsn(), pg_last_wal_replay_lsn()) as byte_lag;" \
    > $BACKUP_ROOT/replica_status_current.txt

# 2. Детальная информация о lag
log "Checking detailed lag information..."
psql -h db-replica -U news_user -d news_db \
    -c "SELECT 
        client_addr,
        application_name,
        state,
        sync_state,
        write_lag,
        flush_lag,
        replay_lag
    FROM pg_stat_replication;" \
    >> $BACKUP_ROOT/replica_status_current.txt

# 3. Проверка, что реплика действительно в режиме только чтения
log "Verifying read-only mode..."
READ_ONLY_CHECK=$(psql -h db-replica -U news_user -d news_db -t -c "SELECT pg_is_in_recovery();")
if [ "$READ_ONLY_CHECK" != "t" ]; then
    log "❌ CRITICAL: Replica is NOT in recovery mode!"
    echo "CRITICAL: Replica not in recovery mode" >> $BACKUP_ROOT/replica_status_current.txt
else
    log "✅ Replica is properly in read-only mode"
fi

# 4. Попытка записи (должна失败)
log "Testing write protection..."
WRITE_TEST=$(psql -h db-replica -U news_user -d news_db -t -c "INSERT INTO test_write (data) VALUES ('test');" 2>&1 | grep -c "read-only" || true)
if [ "$WRITE_TEST" -eq 0 ]; then
    log "⚠️ WARNING: Replica might allow writes!"
else
    log "✅ Write protection is working"
fi

# 5. Сохраняем исторический статус
cp $BACKUP_ROOT/replica_status_current.txt $BACKUP_ROOT/logs/replica_status_${DATE}.txt

log "Replica monitoring completed successfully"

# Ротация логов мониторинга (храним 30 дней)
find $BACKUP_ROOT/logs -name "replica_monitor_*.log" -mtime +30 -delete
find $BACKUP_ROOT/logs -name "replica_status_*.txt" -mtime +30 -delete