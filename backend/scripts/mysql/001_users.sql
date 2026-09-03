CREATE DATABASE IF NOT EXISTS `blog`
  DEFAULT CHARACTER SET utf8mb4
  COLLATE utf8mb4_general_ci;

USE `blog`;

CREATE TABLE IF NOT EXISTS `users` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '用户 ID',
  `nickname` VARCHAR(50) NOT NULL COMMENT '用户昵称',
  `phone` VARCHAR(50) NOT NULL COMMENT '手机号',
  `password` VARCHAR(255) NOT NULL COMMENT '密码摘要',
  `avatar` VARCHAR(255) NULL COMMENT '头像 URL',
  `role` TINYINT NOT NULL DEFAULT 1 COMMENT '用户角色：1-普通用户，2-管理员',
  `created_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '最后修改时间',
  `status` TINYINT NOT NULL COMMENT '用户状态：0-删除，1-正常',
  `last_login_ip` VARCHAR(50) NOT NULL DEFAULT '' COMMENT '最后登录 IP',
  `last_login_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '最后登录时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uni_phone` (`phone`),
  UNIQUE KEY `uni_nickname` (`nickname`),
  KEY `idx_created_time` (`created_time`),
  KEY `idx_updated_time` (`updated_time`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='用户表';
