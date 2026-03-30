// Package config holds application-wide configuration constants.
package config

const (
	KafkaBootstrap    = "localhost:9092"
	SchemaRegistryURL = "http://localhost:8081"
	PostgresDSN       = "postgres://news_user:news_pass@localhost:5432/news_db"
	MongoURI          = "mongodb://news_app:app_password@localhost:27017/news_aggregator"

	// Topics — input
	TopicPostsCreated  = "news.posts.created"
	TopicPostsUpdated  = "news.posts.updated"
	TopicCommentsAdded = "news.comments.added"
	TopicTagsTrending  = "news.tags.trending"

	// Topics — Streams output
	TopicPostsNormalized     = "news.posts.normalized"
	TopicTagsAggregated      = "news.tags.aggregated"
	TopicTagsTrendingHourly  = "news.tags.trending.hourly"

	// Dead Letter Queue
	TopicDLQ = "news.posts.dlq"

	// Consumer group IDs
	GroupPostgresWriter = "cg-postgres-writer"
	GroupMongoEnricher  = "cg-mongo-enricher"
)
