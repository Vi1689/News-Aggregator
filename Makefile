
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
