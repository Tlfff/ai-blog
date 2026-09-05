USE `blog`;

CREATE TABLE IF NOT EXISTS `comments` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '评论 ID',
  `article_id` BIGINT UNSIGNED NOT NULL COMMENT '文章 ID',
  `user_id` BIGINT UNSIGNED NOT NULL COMMENT '评论用户 ID',
  `reply_to_user_id` BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '被回复用户 ID',
  `content` TEXT NOT NULL COMMENT '评论内容',
  `root_id` BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '根评论 ID，主评论为 0',
  `ip` VARCHAR(50) NOT NULL DEFAULT '' COMMENT '评论来源 IP',
  `like_count` BIGINT NOT NULL DEFAULT 0 COMMENT '点赞数投影',
  `comment_count` BIGINT NOT NULL DEFAULT 0 COMMENT '根评论回复数',
  `status` TINYINT NOT NULL DEFAULT 1 COMMENT '评论状态：0-删除，1-正常',
  `created_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  KEY `idx_articleid_rootid_status` (`article_id`, `root_id`, `status`),
  KEY `idx_rootid_status` (`root_id`, `status`),
  KEY `idx_created_time` (`created_time`),
  KEY `idx_updated_time` (`updated_time`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='评论表';
