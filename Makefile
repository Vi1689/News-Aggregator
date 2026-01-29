
up:
	docker-compose up --build -d

# поднятие с выводом в консоль вывод в консоль
up-verbose:
	docker-compose up --build

down:
	docker-compose down

check-mongo-replicas:
	@echo "проверка статуса реплика сета"
	docker exec -it mongodb mongosh --username admin --password mongopass --eval "rs.status()"

	@echo -e "============== проверка статуса главного узла"
	docker exec -it mongodb mongosh --username admin --password mongopass --eval "db.runCommand({ isMaster: 1 })"
	
	@echo -e "============== проверка статуса главного узла"
	docker exec -it mongodb-secondary-1 mongosh --username admin --password mongopass --eval "db.runCommand({ isMaster: 1 })"

	@echo -e "============== проверка статуса главного узла"
	docker exec -it mongodb-secondary-2 mongosh --username admin --password mongopass --eval "db.runCommand({ isMaster: 1 })"

	@echo "============== Проверка размеров БД в репликасете ==="
	@for node in mongodb mongodb-secondary-1 mongodb-secondary-2; do \
		echo "--- $$node ---"; \
		echo "Статус узла:"; \
		docker exec -it $$node mongosh -u admin -p mongopass --eval "rs.isMaster().ismaster ? print('PRIMARY') : print('SECONDARY')"; \
		echo "Размеры БД:"; \
		docker exec -it $$node mongosh -u admin -p mongopass --eval " \
		const db = db.getSiblingDB('news_aggregator'); \
		const stats = db.stats(); \
		print('Данные: ' + (stats.dataSize / 1048576).toFixed(1) + ' MB'); \
		print('Хранилище: ' + (stats.storageSize / 1048576).toFixed(1) + ' MB'); \
		print('Индексы: ' + (stats.indexSize / 1048576).toFixed(1) + ' MB'); \
		print('Документы: ' + stats.objects); \
		"; \
		echo ""; \
	done
