package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

// 8. Поиск по цепочке через общих читателей
func (h *Handler) GetReaderChain(c *gin.Context) {
	userID := c.Param("user_id")
	maxDepth, _ := strconv.Atoi(c.DefaultQuery("max_depth", "3"))

	ctx := c.Request.Context()
	session := h.Neo4jClient.Driver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: h.Neo4jClient.Database})
	defer session.Close(ctx)

	query := `
		MATCH path = (u:User {id: $user_id})-[:SHARED_TO*1..$max_depth]->(other:User)
		RETURN other.id AS user_id,
			   length(path) AS chain_length,
			   [rel IN relationships(path) | rel.platform] AS platforms
		ORDER BY chain_length ASC
		LIMIT 50
	`

	result, err := session.Run(ctx, query, map[string]interface{}{
		"user_id":   userID,
		"max_depth": maxDepth,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var users []map[string]interface{}
	for result.Next(ctx) {
		record := result.Record()
		users = append(users, map[string]interface{}{
			"user_id":      record.Values[0],
			"chain_length": record.Values[1],
			"platforms":    record.Values[2],
		})
	}

	c.JSON(http.StatusOK, gin.H{"reader_chain": users})
}

// 9. Рекомендации на основе просмотров других пользователей
func (h *Handler) GetCollaborativeRecommendations(c *gin.Context) {
	userID := c.Param("user_id")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))

	ctx := c.Request.Context()
	session := h.Neo4jClient.Driver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: h.Neo4jClient.Database})
	defer session.Close(ctx)

	query := `
		// Пользователи, которые читали те же новости
		MATCH (u:User {id: $user_id})-[:VIEWED]->(n:News)<-[:VIEWED]-(other:User)
		WHERE u.id <> other.id
		
		// Новости, которые читали другие пользователи, но не читал текущий
		MATCH (other)-[:VIEWED]->(rec:News)
		WHERE NOT EXISTS((u)-[:VIEWED]->(rec))
		
		RETURN rec.id AS news_id,
			   rec.title AS title,
			   count(DISTINCT other) AS common_users,
			   avg([(u2)-[:VIEWED]->(rec) | u2.id]) AS viewers
		ORDER BY common_users DESC
		LIMIT $limit
	`

	result, err := session.Run(ctx, query, map[string]interface{}{
		"user_id": userID,
		"limit":   limit,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var recommendations []map[string]interface{}
	for result.Next(ctx) {
		record := result.Record()
		recommendations = append(recommendations, map[string]interface{}{
			"news_id":      record.Values[0],
			"title":        record.Values[1],
			"common_users": record.Values[2],
			"viewers":      record.Values[3],
		})
	}

	c.JSON(http.StatusOK, gin.H{"collaborative_recommendations": recommendations})
}
