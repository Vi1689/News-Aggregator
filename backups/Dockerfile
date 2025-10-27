FROM postgres:15

# Устанавливаем cron и docker-cli
RUN apt-get update && apt-get install -y \
    cron \
    docker.io \
    curl \
    && rm -rf /var/lib/apt/lists/*

# Создаем директории
RUN mkdir -p /backups/{scripts,data,daily,weekly,monthly,logs}

# Копируем скрипты
COPY scripts/ /backups/scripts/

# Делаем скрипты исполняемыми
RUN chmod +x /backups/scripts/*.sh

# Устанавливаем cron задачу
RUN crontab /backups/scripts/crontab.config

CMD ["sh", "-c", "echo '=== Backup Service Started ===' && /backups/scripts/backup-master.sh && echo 'Starting cron...' && cron -f -L /backups/logs/cron.log"]