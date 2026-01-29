# [file name]: db/init-mongo-replica.sh
#!/bin/bash
set -e

echo "Waiting for MongoDB instances to be ready..."
sleep 10

echo "Initializing replica set..."

# Инициализация replica set
mongosh --host mongodb-primary:27017 -u admin -p mongopass --authenticationDatabase admin --eval '
  rs.initiate({
    _id: "rs0",
    members: [
      {_id: 0, host: "mongodb-primary:27017", priority: 2},
      {_id: 1, host: "mongodb-secondary1:27017", priority: 1},
      {_id: 2, host: "mongodb-secondary2:27017", priority: 1}
    ]
  })
'

echo "Waiting for replica set to be ready..."
sleep 10

echo "Checking replica set status..."
mongosh --host mongodb-primary:27017 -u admin -p mongopass --authenticationDatabase admin --eval 'rs.status()'

echo "Creating user for application..."
mongosh --host mongodb-primary:27017 -u admin -p mongopass --authenticationDatabase admin --eval '
  db = db.getSiblingDB("news_aggregator");
  db.createUser({
    user: "news_app",
    pwd: "app_password",
    roles: [
      {
        role: "readWrite",
        db: "news_aggregator"
      }
    ]
  });
'

echo "Replica set initialization complete!"