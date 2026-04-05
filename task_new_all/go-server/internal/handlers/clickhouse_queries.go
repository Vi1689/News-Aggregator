package handlers

// // 5. Retention пользователей
// func (h *Handler) GetUserRetention(c *gin.Context) {
// 	days, _ := strconv.Atoi(c.DefaultQuery("days", "30"))

// 	query := `
// 		WITH
// 			first_activity AS (
// 				SELECT userId, min(toDate(timestamp)) AS first_date
// 				FROM news_events_raw
// 				WHERE userId != '' AND eventType = 'NewsViewed'
// 				GROUP BY userId
// 			),
// 			daily_activity AS (
// 				SELECT DISTINCT toDate(timestamp) AS date, userId
// 				FROM news_events_raw
// 				WHERE userId != '' AND eventType = 'NewsViewed'
// 			)
// 		SELECT
// 			first_date,
// 			count(DISTINCT fa.userId) AS new_users,
// 			count(DISTINCT da.userId) AS active_users,
// 			round(active_users * 100.0 / new_users, 2) AS retention_rate
// 		FROM first_activity fa
// 		LEFT JOIN daily_activity da ON fa.userId = da.userId AND da.date = fa.first_date + INTERVAL 1 DAY
// 		WHERE first_date >= today() - $1
// 		GROUP BY first_date
// 		ORDER BY first_date DESC
// 	`

// 	ctx := c.Request.Context()
// 	rows, err := h.ClickHouseClient.Conn.Query(ctx, query, days)
// 	if err != nil {
// 		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
// 		return
// 	}
// 	defer rows.Close()

// 	var results []map[string]interface{}
// 	for rows.Next() {
// 		var firstDate time.Time
// 		var newUsers, activeUsers int
// 		var retentionRate float64

// 		if err := rows.Scan(&firstDate, &newUsers, &activeUsers, &retentionRate); err != nil {
// 			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
// 			return
// 		}

// 		results = append(results, map[string]interface{}{
// 			"date":           firstDate,
// 			"new_users":      newUsers,
// 			"active_users":   activeUsers,
// 			"retention_rate": retentionRate,
// 		})
// 	}

// 	c.JSON(http.StatusOK, gin.H{"retention": results})
// }

// // 6. Анализ вовлечённости по времени суток
// func (h *Handler) GetEngagementByHour(c *gin.Context) {
// 	query := `
// 		SELECT
// 			toHour(timestamp) AS hour,
// 			eventType,
// 			count() AS events,
// 			uniq(userId) AS unique_users,
// 			uniq(newsId) AS unique_news
// 		FROM news_events_raw
// 		WHERE timestamp >= today() - 7
// 		  AND eventType IN ('NewsViewed', 'NewsLiked', 'NewsShared')
// 		GROUP BY hour, eventType
// 		ORDER BY hour ASC, eventType
// 	`

// 	ctx := c.Request.Context()
// 	rows, err := h.ClickHouseClient.Conn.Query(ctx, query)
// 	if err != nil {
// 		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
// 		return
// 	}
// 	defer rows.Close()

// 	var results []map[string]interface{}
// 	for rows.Next() {
// 		var hour int
// 		var eventType string
// 		var events, uniqueUsers, uniqueNews int

// 		if err := rows.Scan(&hour, &eventType, &events, &uniqueUsers, &uniqueNews); err != nil {
// 			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
// 			return
// 		}

// 		results = append(results, map[string]interface{}{
// 			"hour":         hour,
// 			"event_type":   eventType,
// 			"events":       events,
// 			"unique_users": uniqueUsers,
// 			"unique_news":  uniqueNews,
// 		})
// 	}

// 	c.JSON(http.StatusOK, gin.H{"engagement_by_hour": results})
// }
