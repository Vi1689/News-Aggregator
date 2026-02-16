.PHONY: up down logs restart status clean monitoring

up:
	docker-compose -f docker-compose.yml -f docker-compose.monitoring.yml up -d

down:
	docker-compose -f docker-compose.yml -f docker-compose.monitoring.yml down

logs:
	docker-compose -f docker-compose.yml -f docker-compose.monitoring.yml logs -f

restart:
	docker-compose -f docker-compose.yml -f docker-compose.monitoring.yml restart

status:
	docker-compose -f docker-compose.yml -f docker-compose.monitoring.yml ps

clean:
	docker-compose -f docker-compose.yml -f docker-compose.monitoring.yml down -v

monitoring-up:
	docker-compose -f docker-compose.monitoring.yml up -d

monitoring-down:
	docker-compose -f docker-compose.monitoring.yml down

grafana:
	open http://localhost:3000

prometheus:
	open http://localhost:9090

export-data:
	mkdir -p exports
	docker exec -t postgres-master pg_dump -U news_user -d news_db > exports/backup_$(date +%Y%m%d_%H%M%S).sql
	docker exec -t mongodb mongodump --username admin --password mongopass --authenticationDatabase admin --out /tmp/mongodump
	docker cp mongodb:/tmp/mongodump exports/mongodump_$(date +%Y%m%d_%H%M%S)

health-check:
	@echo "=== Health Check ==="
	@curl -s http://localhost:8080/health | jq .
	@echo "\n=== Prometheus ==="
	@curl -s http://localhost:9090/-/healthy
	@echo "\n=== Grafana ==="
	@curl -s http://localhost:3000/api/health | jq .
	@echo "\n=== PostgreSQL Exporter ==="
	@curl -s http://localhost:9187/metrics | head -5
	@echo "\n=== MongoDB Exporter ==="
	@curl -s http://localhost:9216/metrics | head -5