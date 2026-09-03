package po

type Book struct {
	Id                     uint    `xorm:"'id' autoincr pk"`
	Title                  string  `xorm:"'title' varchar(100) notnull default ''"`
	AliasTitle             string  `xorm:"'alias_title' varchar(255) notnull default ''"`
	StacksBookId           uint    `xorm:"'stacks_book_id' notnull"`
	SourceId               uint    `xorm:"'source_id' notnull default 0"`
	ApiBookId              uint64  `xorm:"'api_book_id' notnull default 0"`
	WordsNum               uint    `xorm:"'words_num' notnull default 0"`
	Author                 string  `xorm:"'author' varchar(50) notnull default ''"`
	AuthorId               uint    `xorm:"'author_id' notnull default 0"`
	Characters             string  `xorm:"'characters' varchar(100) notnull default ''"`
	IsOver                 uint8   `xorm:"'is_over' notnull default 0"`
	Category1              uint    `xorm:"'category1' notnull default 0"`
	Category2              uint    `xorm:"'category2' notnull default 0"`
	ImageLink              string  `xorm:"'image_link' varchar(100) notnull default ''"`
	LatestChapterId        uint64  `xorm:"'latest_chapter_id' notnull default 0"`
	LatestChapterTitle     string  `xorm:"'latest_chapter_title' varchar(100) notnull default ''"`
	LatestChapterUrl       string  `xorm:"'latest_chapter_url' varchar(100) notnull default ''"`
	TotalChapterNum        int16   `xorm:"'total_chapter_num' notnull default 0"`
	OrderNum               uint    `xorm:"'order_num' notnull default 0"`
	RankWeek               uint    `xorm:"'rank_week' notnull default 0"`
	RankMonth              uint    `xorm:"'rank_month' notnull default 0"`
	ChapterVer             uint    `xorm:"'chapter_ver' notnull default 0"`
	ChapterTitleFormat     uint8   `xorm:"'chapter_title_format' notnull default 0"`
	IsBreak                uint8   `xorm:"'is_break' notnull default 0"`
	IsLock                 uint8   `xorm:"'is_lock' notnull"`
	IsClassical            uint8   `xorm:"'is_classical' notnull"`
	IsUp                   uint8   `xorm:"'is_up' notnull default 0"`
	IsWhite                uint8   `xorm:"'is_white' notnull default 0"`
	UpdateTime             int     `xorm:"'update_time' default 0"`
	Status                 uint16  `xorm:"'status' notnull default 0"`
	SubTreasury            uint16  `xorm:"'sub_treasury' notnull default 4"`
	SubTreasuryTime        uint    `xorm:"'sub_treasury_time' notnull default 0"`
	CreatedAt              uint    `xorm:"'created_at' notnull default 0"`
	UpdatedAt              uint    `xorm:"'updated_at' notnull"`
	UpTime                 uint    `xorm:"'up_time' notnull default 0"`
	Level                  uint16  `xorm:"'level' notnull default 0"`
	Level2                 uint16  `xorm:"'level2' notnull default 0"`
	IsTest                 uint16  `xorm:"'is_test' notnull default 0"`
	SensitiveType          uint16  `xorm:"'sensitive_type' notnull default 1"`
	Score                  float32 `xorm:"'score' decimal(4,1) notnull default 7.5"`
	Score2                 float32 `xorm:"'score2' decimal(4,1) notnull default 7.5"`
	InitScore              float32 `xorm:"'init_score' decimal(4,1) notnull default 0.0"`
	Count                  uint    `xorm:"'count' notnull default 0"`
	DominantHue            string  `xorm:"'dominant_hue' varchar(20) notnull default ''"`
	EachAudit              uint8   `xorm:"'each_audit' notnull default 0"`
	Price                  float32 `xorm:"'price' decimal(10,2) notnull default 0.00"`
	IsHidden               uint8   `xorm:"'is_hidden' notnull default 0"`
	LatestChapterUpdatedAt int     `xorm:"'latest_chapter_updated_at' notnull default 0"`
	CurrentFlowPool        uint16  `xorm:"'current_flow_pool' notnull default 0"`
	CurrentFlowPoolTime    uint    `xorm:"'current_flow_pool_time' notnull default 0"`
	RightFlowPool          uint16  `xorm:"'right_flow_pool' notnull default 0"`
	RewardStatus           uint16  `xorm:"'reward_status' notnull default 0"`
	LevelNew               uint    `xorm:"'level_new' notnull default 0"`
	LevelPotential         uint    `xorm:"'level_potential' notnull default 0"`
	LevelPublish           uint    `xorm:"'level_publish' notnull default 0"`
	LevelPublishPotential  uint    `xorm:"'level_publish_potential' notnull default 0"`
}

func (Book) TableName() string {
	return "book"
}

/*CREATE TABLE `book` (
  `id` int(11) unsigned NOT NULL AUTO_INCREMENT,
  `title` varchar(100) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NOT NULL DEFAULT '' COMMENT '书籍名称',
  `alias_title` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NOT NULL DEFAULT '' COMMENT '书籍别名',
  `stacks_book_id` int(10) unsigned NOT NULL COMMENT '对应的书库的书籍id',
  `source_id` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '作品所属源站',
  `api_book_id` bigint(20) unsigned NOT NULL DEFAULT '0' COMMENT '合作方cp的书籍id',
  `words_num` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '书籍总字数',
  `author` varchar(50) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NOT NULL DEFAULT '' COMMENT '作者名称',
  `author_id` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '作者id',
  `characters` varchar(100) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NOT NULL DEFAULT '' COMMENT '书籍中主角的名称',
  `is_over` tinyint(1) unsigned NOT NULL DEFAULT '0' COMMENT '是否完结（0：未完结    1：已完结）',
  `category1` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '一级分类id',
  `category2` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '二级分类id',
  `image_link` varchar(100) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NOT NULL DEFAULT '' COMMENT '书籍封面',
  `latest_chapter_id` bigint(20) NOT NULL DEFAULT '0' COMMENT '最新章节id',
  `latest_chapter_title` varchar(100) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NOT NULL DEFAULT '' COMMENT '最新章节的名称',
  `latest_chapter_url` varchar(100) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NOT NULL DEFAULT '' COMMENT '最新章节的跳转地址',
  `total_chapter_num` smallint(6) NOT NULL DEFAULT '0' COMMENT '总章节数',
  `order_num` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '排序',
  `rank_week` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '周点击量',
  `rank_month` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '月点击量',
  `chapter_ver` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '章节的版本',
  `chapter_title_format` tinyint(1) unsigned NOT NULL DEFAULT '0' COMMENT '是否格式化章节标题:0-否,1-是',
  `is_break` tinyint(1) unsigned NOT NULL DEFAULT '0' COMMENT '是否断更（0：未断更    1：已断更）',
  `is_lock` tinyint(1) unsigned NOT NULL COMMENT '是否锁定（0：未锁定    1：人工锁定  2：系统锁定）',
  `is_classical` tinyint(1) unsigned NOT NULL COMMENT '是否经典（0：不是      1：是经典）',
  `is_up` tinyint(1) unsigned NOT NULL DEFAULT '0' COMMENT '是否上架（0-下架    1-上架）',
  `is_white` tinyint(1) unsigned NOT NULL DEFAULT '0' COMMENT '是否白名单（0-不是白名单,1-白名单）',
  `update_time` int(11) DEFAULT '0' COMMENT '最后更新时间',
  `status` smallint(3) unsigned NOT NULL DEFAULT '0' COMMENT '状态（0:未审核；1：正常；2：回收站）',
  `sub_treasury` smallint(3) unsigned NOT NULL DEFAULT '4' COMMENT '分库 1:测试库; 2:降权库; 3:核心推荐库; 4:一般推荐库',
  `sub_treasury_time` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '进入分库的时间',
  `created_at` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '创建时间',
  `updated_at` int(10) unsigned NOT NULL COMMENT '更新时间',
  `up_time` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '上架时间',
  `level` smallint(4) unsigned NOT NULL DEFAULT '0' COMMENT '等级（100-S,90-A,80-B,70-C,50-J,40-L,0-未定义)',
  `level2` smallint(3) unsigned NOT NULL DEFAULT '0' COMMENT '评级2',
  `is_test` smallint(3) unsigned NOT NULL DEFAULT '0' COMMENT '是否测试新书：1是，0否',
  `sensitive_type` smallint(3) unsigned NOT NULL DEFAULT '1' COMMENT '敏感程度 1未核查 2正常 3轻度 4重度 5中度',
  `score` decimal(4,1) unsigned NOT NULL DEFAULT '7.5' COMMENT '分数',
  `score2` decimal(4,1) unsigned NOT NULL DEFAULT '7.5' COMMENT '分数V2',
  `init_score` decimal(4,1) unsigned NOT NULL DEFAULT '0.0' COMMENT '书籍脚本评分',
  `count` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '被评为L级的次数',
  `dominant_hue` varchar(20) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL DEFAULT '' COMMENT '色调',
  `each_audit` tinyint(3) unsigned NOT NULL DEFAULT '0' COMMENT '是否逐章审核',
  `price` decimal(10,2) unsigned NOT NULL DEFAULT '0.00' COMMENT '价格',
  `is_hidden` tinyint(1) unsigned NOT NULL DEFAULT '0' COMMENT '是否隐藏（0：未隐藏 1：隐藏)',
  `latest_chapter_updated_at` int(11) NOT NULL DEFAULT '0' COMMENT '正式库最新章节接口更新时间',
  `current_flow_pool` smallint(4) unsigned NOT NULL DEFAULT '0' COMMENT '当前流量池; 1 ~ 5: 一至五级流量池; 11: 降权库; 12: 新书测试库;',
  `current_flow_pool_time` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '入流量池时间',
  `right_flow_pool` smallint(4) unsigned NOT NULL DEFAULT '0' COMMENT '正确流量池; 1 ~ 5: 一至五级流量池; 11: 降权库; 12: 新书测试库;',
  `reward_status` smallint(4) unsigned NOT NULL DEFAULT '0' COMMENT '打赏状态：0-关闭;1-开启;',
  `level_new` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '新评级',
  `level_potential` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '潜力评级',
  `level_publish` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '图书评级',
  `level_publish_potential` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '图书潜力评级',
  PRIMARY KEY (`id`),
  UNIQUE KEY `source_book` (`source_id`,`api_book_id`),
  UNIQUE KEY `stacks_book_id` (`stacks_book_id`),
  KEY `category` (`category1`,`category2`),
  KEY `idx_author` (`author`),
  KEY `title` (`title`)
) ENGINE=InnoDB AUTO_INCREMENT=216752 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci ROW_FORMAT=DYNAMIC COMMENT='书籍表';

*/
