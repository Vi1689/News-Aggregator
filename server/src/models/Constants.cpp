#include "Constants.h"

namespace constants {
const std::vector<std::string> CONN_STRINGS = {
    "host=db-master port=5432 dbname=news_db user=news_user password=news_pass",
    "host=db-replica port=5432 dbname=news_db user=news_user "
    "password=news_pass"};

const size_t POOL_SIZE = 4; // Размер пула соединений

const std::unordered_map<std::string, std::string> pk_map = {
    {"users", "user_id"},       {"authors", "author_id"},
    {"news_texts", "text_id"},  {"sources", "source_id"},
    {"channels", "channel_id"}, {"posts", "post_id"},
    {"media", "media_id"},      {"tags", "tag_id"},
    {"comments", "comment_id"}};

const std::vector<std::string> valid_tables = {"users",
                                               "authors",
                                               "news_texts",
                                               "sources",
                                               "channels",
                                               "posts",
                                               "media",
                                               "tags",
                                               "post_tags",
                                               "comments",
                                               "channel_activity_stats",
                                               "author_performance",
                                               "tag_popularity_detailed",
                                               "source_post_stats",
                                               "user_comment_activity",
                                               "posts_ranked_by_popularity",
                                               "author_likes_trend",
                                               "cumulative_posts_analysis",
                                               "tag_rank_by_channel",
                                               "commenter_analysis",
                                               "posts_with_detailed_authors",
                                               "channels_with_sources",
                                               "posts_with_authors_and_texts",
                                               "comments_with_post_info",
                                               "posts_with_tags_and_channels",
                                               "media_with_context",
                                               "comprehensive_post_info",
                                               "extended_post_analytics"};

bool is_valid_table(const std::string &t) {
  for (const auto &x : valid_tables)
    if (x == t)
      return true;
  return false;
}
} // namespace constants