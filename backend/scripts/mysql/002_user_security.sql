USE `blog`;

CREATE TABLE IF NOT EXISTS `user_session_cleanup_tasks` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '补偿任务 ID',
  `user_id` BIGINT UNSIGNED NOT NULL COMMENT '用户 ID',
  `current_token` VARCHAR(128) NOT NULL COMMENT '应保留的当前设备 Token',
  `status` TINYINT NOT NULL DEFAULT 1 COMMENT '任务状态：1-待处理；2-已完成',
  `created_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`), KEY `idx_status_id` (`status`, `id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='用户会话收敛补偿任务';
