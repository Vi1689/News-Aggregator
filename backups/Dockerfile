FROM postgres:15

RUN apt-get update && apt-get install -y curl && rm -rf /var/lib/apt/lists/* && apt-get clean

RUN mkdir -p /backups/{scripts,data,logs}

COPY scripts/ /backups/scripts/

RUN chmod +x /backups/scripts/*.sh

CMD ["/bin/bash", "-c", "echo '=== Database Backup Service Started ===' && while true; do /backups/scripts/backup-master.sh; echo 'Sleeping for 24 hours...'; sleep 86400; done"]