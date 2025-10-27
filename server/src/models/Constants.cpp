#include "Constants.h"

namespace constants {
    const std::vector<std::string> CONN_STRINGS = {
        "host=db-master port=5432 dbname=news_db user=news_user password=news_pass",
        "host=db-replica port=5432 dbname=news_db user=news_user password=news_pass"
    };

    const std::unordered_map<std::string, std::string> pk_map = {
        {"users", "user_id"},       {"authors", "author_id"},
        {"news_texts", "text_id"},  {"sources", "source_id"},
        {"channels", "channel_id"}, {"posts", "post_id"},
        {"media", "media_id"},      {"tags", "tag_id"},
        {"comments", "comment_id"}
    };

    const std::vector<std::string> valid_tables = {
        "users", "authors", "news_texts", "sources", "channels", 
        "posts", "media", "tags", "post_tags", "comments",
        "top_authors", "active_users", "popular_tags", "posts_by_channel",
        "avg_comments_per_post", "posts_ranked", "comments_moving_avg",
        "cumulative_posts", "tag_rank", "user_activity_rank",
        "posts_with_authors", "comments_with_users", "posts_with_tags",
        "posts_authors_channels", "comments_posts_users", "posts_authors_tags",
        "full_post_info", "full_post_media"
    };

    bool is_valid_table(const std::string &t) {
        for (const auto &x : valid_tables)
            if (x == t) return true;
        return false;
    }
}