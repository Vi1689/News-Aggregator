#!/bin/bash
set -e

BACKUP_ROOT="/backups"
DATE=$(date +%Y%m%d_%H%M%S)

export PGPASSWORD="news_pass"

log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1" | tee -a $BACKUP_ROOT/logs/replica_status_${DATE}.log
}

log "Checking replica status..."

# Проверка статуса репликации
psql -h db-replica -U news_user -d news_db \
    -c "SELECT 
        now() as current_time, 
        pg_is_in_recovery() as is_replica,
        pg_last_wal_receive_lsn() as receive_lsn,
        pg_last_wal_replay_lsn() as replay_lsn,
        pg_last_xact_replay_timestamp() as replay_time,
        now() - pg_last_xact_replay_timestamp() AS replication_lag;" \
    >> $BACKUP_ROOT/logs/replica_status_${DATE}.log

log "Replica status check completed"