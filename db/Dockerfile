# [file name]: Dockerfile.mongo-replica-init
FROM mongo:7

COPY ./db/init-mongo-replica.sh /docker-entrypoint-initdb.d/
RUN chmod +x /docker-entrypoint-initdb.d/init-mongo-replica.sh