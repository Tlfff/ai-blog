USE `blog`;

CREATE TABLE IF NOT EXISTS `comment_likes` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '点赞关系 ID',
  `user_id` BIGINT UNSIGNED NOT NULL COMMENT '点赞用户 ID',
  `comment_id` BIGINT UNSIGNED NOT NULL COMMENT '评论 ID',
  `status` TINYINT NOT NULL DEFAULT 1 COMMENT '点赞状态：1-点赞；2-未点赞',
  `created_time` DATETIME(6) NOT NULL COMMENT '关系创建时间',
  `updated_time` DATETIME(6) NOT NULL COMMENT '关系更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uni_comment_like_user_comment` (`user_id`, `comment_id`),
  KEY `idx_comment_like_comment_status` (`comment_id`, `status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='评论点赞事实表';

CREATE TABLE IF NOT EXISTS `comment_like_event_outbox` (
  `event_id` VARCHAR(64) NOT NULL COMMENT '集成事件幂等标识',
  `aggregate_id` BIGINT UNSIGNED NOT NULL COMMENT '点赞关系 ID',
  `event_type` VARCHAR(64) NOT NULL COMMENT '稳定事件类型',
  `version` BIGINT NOT NULL COMMENT '点赞关系单调版本',
  `occurred_at` DATETIME(6) NOT NULL COMMENT '点赞事实发生时间',
  `payload` JSON NOT NULL COMMENT '集成事件 JSON 负载',
  `status` TINYINT NOT NULL DEFAULT 0 COMMENT '发布状态：0-待发布；1-已发布',
  `attempts` INT UNSIGNED NOT NULL DEFAULT 0 COMMENT '发布失败次数',
  `next_attempt_time` DATETIME(6) NOT NULL COMMENT '下次允许发布时间',
  `published_time` DATETIME(6) NULL COMMENT '发布成功时间',
  `last_error` TEXT NOT NULL COMMENT '最近一次发布失败原因',
  `created_time` DATETIME(6) NOT NULL COMMENT 'Outbox 创建时间',
  `updated_time` DATETIME(6) NOT NULL COMMENT 'Outbox 更新时间',
  PRIMARY KEY (`event_id`),
  UNIQUE KEY `uni_comment_like_outbox_version` (`aggregate_id`, `version`),
  KEY `idx_comment_like_outbox_pending` (`status`, `next_attempt_time`, `created_time`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='评论点赞集成事件事务 Outbox';

CREATE TABLE IF NOT EXISTS `comment_like_projection_lock` (
  `id` TINYINT UNSIGNED NOT NULL COMMENT '固定协调行 ID',
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='评论点赞投影重建协调锁';

INSERT IGNORE INTO `comment_like_projection_lock` (`id`) VALUES (1);

CREATE TABLE IF NOT EXISTS `comment_like_event_inbox` (
  `event_id` VARCHAR(64) NOT NULL COMMENT '已处理事件幂等标识',
  `like_id` BIGINT UNSIGNED NOT NULL COMMENT '点赞关系 ID',
  `comment_id` BIGINT UNSIGNED NOT NULL COMMENT '评论 ID',
  `processed_time` DATETIME(6) NOT NULL COMMENT '事务处理完成时间',
  PRIMARY KEY (`event_id`),
  KEY `idx_comment_like_inbox_like` (`like_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='评论点赞计数消费者 Inbox';

CREATE TABLE IF NOT EXISTS `comment_like_projection` (
  `like_id` BIGINT UNSIGNED NOT NULL COMMENT '点赞关系 ID',
  `comment_id` BIGINT UNSIGNED NOT NULL COMMENT '评论 ID',
  `user_id` BIGINT UNSIGNED NOT NULL COMMENT '点赞用户 ID',
  `version` BIGINT NOT NULL COMMENT '最后应用的点赞关系版本',
  `active` TINYINT NOT NULL COMMENT '是否计入点赞数：0-否；1-是',
  `last_event_id` VARCHAR(64) NOT NULL COMMENT '最后应用的事件 ID',
  `updated_time` DATETIME(6) NOT NULL COMMENT '投影更新时间',
  PRIMARY KEY (`like_id`),
  UNIQUE KEY `uni_comment_like_projection_user_comment` (`user_id`, `comment_id`),
  KEY `idx_comment_like_projection_comment` (`comment_id`, `active`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='评论上下文的点赞状态投影';
