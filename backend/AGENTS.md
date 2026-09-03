# AGENTS.md

本文件用于约束 AI 在后端项目中的代码分析、设计、编写、修改、测试和交付行为。其作用范围为 `backend/` 及全部子目录。

## 1. 基本原则

1. 所有沟通、代码说明和新增文档默认使用中文；代码标识符、协议字段及行业通用术语保持其标准英文写法。
2. 修改代码前先阅读相关实现、测试、配置和文档，确认调用链及影响范围，不凭猜测直接改动。
3. 只修改完成当前任务所必需的内容，禁止顺手重构、批量改名、调整无关格式或删除不理解的代码。
4. 优先遵循仓库现有架构和代码风格；若现有实现与本文件冲突，新增及本次修改的代码以本文件为准，存量代码不要求无关地批量整改。
5. 不得直接修改自动生成文件，包括 `api/` 下的 `*.pb.go`、`*_grpc.pb.go`、`*_http.pb.go`、`*.pb.validate.go`，以及 `internal/conf/conf.pb.go`、`internal/domain/**/meta/*_enum.pb.go`、`cmd/**/wire_gen.go`。需要变更时，应修改对应的 `.proto`、`wire.go` 或 ProviderSet，并通过 `make api`、`make config`、`make gen` 重新生成。
6. 不得提交密钥、Token、密码、生产环境地址或其他敏感信息；示例配置必须使用安全的占位值。
7. 未经明确要求，不改变公开 API、数据库结构、消息格式、配置项语义及既有兼容行为。

## 2. Leo 脚手架架构与实现规范

### 2.1 分层职责

1. 项目统一沿用 Leo 脚手架现有目录，典型业务调用链为 `api 生成路由 -> internal/app -> internal/domain -> internal/domain/<业务>/repo -> internal/clients`。`cmd` 是依赖组装与进程入口，`internal/server` 是传输注册层，二者不得承载业务规则。
2. `api/<业务>` 只存放对外协议的 `.proto` 及其生成代码。HTTP 路由、请求方法、请求体、字段校验和 gRPC 方法优先在 Proto 中声明，不得另建手写 DTO、重复路由定义或手工维护同义协议结构。
3. `internal/app/service` 实现 Proto 生成的 HTTP Controller 或 gRPC Server 接口，负责协议对象与领域对象转换、调用领域模块和组织响应；不得直接操作 MySQL、Redis、Kafka 或编写核心业务规则。
4. `internal/app/job` 和 `internal/app/consumer` 分别作为任务与消息消费的应用入口，只负责编排领域能力。Consumer 的订阅、消息处理与生命周期应接入 Leo Stream，不得自行实现常驻轮询框架。
5. `internal/domain/<业务>` 存放贫血领域实体、领域服务、业务规则和数据访问接口。领域模块不得依赖 `api`、`cmd`、`internal/server` 或具体数据库实现；仓储接口应定义在使用它的领域模块中。
6. `internal/domain/<业务>/repo` 是领域仓储接口的 Adapter；`po` 存放持久化对象，`factory` 负责领域实体与 PO 的转换。数据库查询、缓存读写和持久化细节必须收敛在此，不得泄漏到应用层。
7. `internal/clients` 负责创建和释放 MySQL、Redis 等外部资源客户端，并以 Wire Provider 的形式提供依赖；不得在业务函数中临时创建连接或直接读取配置构造客户端。
8. `internal/server` 只聚合并注册生成的 HTTP/gRPC 接口；`cmd` 只定义 Cobra 命令、Wire 组装、Leo App 和进程启动。禁止在路由注册、命令入口、Wire Provider 中编写业务逻辑。
9. `internal/conf` 负责配置结构和配置源初始化；`internal/middleware` 只存放 Leo 尚未覆盖且确有项目业务语义的中间件；`internal/pkg` 只放不依赖具体业务的内部通用工具。
10. 依赖方向必须保持单向，禁止循环依赖和跨层反向调用。应用层只调用领域模块公开的接口，仓储 Adapter 实现领域层定义的数据访问接口并通过 `internal/clients` 访问外部资源，依赖统一由 Wire 注入。

### 2.2 贫血领域模型约束

1. 项目统一使用贫血领域模型。`internal/domain/<业务>/entity` 中的实体只承载领域数据，不封装业务规则、状态流转、权限判断、持久化操作或外部资源调用。
2. 实体可以包含字段、类型定义及无业务决策的数据转换辅助方法；禁止在实体上实现 `Publish`、`Delete`、`CanEdit`、`Like`、`ValidateStatus` 等包含业务判断或状态修改的方法。
3. 单实体规则、跨实体规则、状态机、权限判断、计数变更、幂等控制和事务编排统一放在 `internal/domain/<业务>` 的领域服务中；应用层只能调用领域服务公开的方法，不得自行复制业务判断。
4. 领域服务接收实体、值参数和领域层定义的仓储接口，通过显式返回值或错误表达处理结果；不得依赖 Proto 请求对象、Gin Context、持久化 PO 或具体客户端类型。
5. `repo/po` 只描述数据库持久化结构，`repo/factory` 只负责 PO 与实体的字段转换；二者不得承载业务校验、状态流转或权限规则。
6. 实体字段允许由领域服务直接读写；为字段机械创建无业务价值的 Getter、Setter 或 Builder 属于无效封装，禁止使用。
7. 业务常量和状态枚举放在领域层并使用明确类型；实体只保存状态值，状态是否允许转换由领域服务判断。
8. 测试应重点覆盖领域服务的正常路径、非法状态、权限、边界及仓储失败场景；实体测试仅覆盖确有必要的纯数据转换，不为简单字段读写编写测试。

### 2.3 Leo 能力复用

1. 新增基础能力前必须先检查 Leo 及项目已有模块。Leo 已提供同类能力时必须直接复用或扩展其配置，不得重复封装一层仅做透传的模块，也不得另行引入功能重叠的框架。
2. **生命周期**：HTTP、gRPC、Consumer、Actuator 等长驻模块必须实现或复用 `leo.Runner`，通过 `leo.NewApp`、`leo.Runners` 和 `App.Run` 统一并发启动与优雅退出；不得自行重复实现信号监听、服务编排和关闭流程。
3. **传输层**：HTTP 使用 `leo/transport/ginhttp`，gRPC 使用 `leo/transport/lgrpc`；服务注册沿用生成代码和 `internal/server`。除非 Leo 明确无法满足需求并记录原因，否则不得直接另建 `http.Server`、`grpc.Server` 或第二套路由框架。
4. **日志**：统一使用 `leo/log` 及项目已有的 `leo/log/slog` Adapter。请求、任务和消息处理中的日志应携带已有 `context.Context`，优先使用 `log.FromContext(ctx)` 或 `log.L().WithContext(ctx)`；业务代码不得使用标准库 `log`、`fmt.Println` 或自行创建全局 Logger。
5. **HTTP 中间件**：优先使用 Leo 提供的 `context`、`log`、`recovery`、`requestid`、`sentry` 中间件，并保持项目既有顺序。不得重复实现请求 ID、访问日志、panic 恢复、链路 Logger 注入和 Sentry 捕获；自定义中间件必须具备明确业务语义并放入 `internal/middleware`。
6. **配置**：统一使用 `leo/config`、`decoder` 以及 `resource/file`、`resource/nacosv2` 加载配置，通过 `config.Get(path).Scan(&target)` 获取配置；不得再引入 Viper、重复的 YAML 解析器或在业务代码中散落读取环境变量，进程启动参数及 Leo、部署脚本约定的环境变量除外。
7. **健康检查与管理端点**：统一使用 Leo Actuator，以及传输 Server 提供的 `ActuatorHandler`、`HealthChecker`；不得重复实现健康检查、服务状态、pprof 或同类管理端点。
8. **消息消费**：统一使用 `leo/stream`、`stream.Streamer` 和 Leo Kafka Adapter 管理订阅、Handler、错误处理及退出；业务 Consumer 只实现订阅配置和 `Handle`，不得自建消费循环。
9. **协议处理**：优先复用 Proto 生成代码已提供的参数解析、校验、错误映射、编码和渲染流程。应用服务返回生成的响应对象与错误，不得在业务方法中再次手工绑定请求或直接写 Gin JSON 响应。
10. **依赖注入**：构造函数和 ProviderSet 使用 Google Wire 管理；新增依赖时修改对应层的 `provider_set.go` 或 `wire.go` 并重新生成 `wire_gen.go`，不得使用 Service Locator、运行时全局容器或在方法内部隐藏创建依赖。
11. Leo 未提供所需能力时，应先复用仓库已有 Adapter；仍无法满足时才允许新增实现，并在代码或交付说明中写明缺口、选型理由以及与 Leo 的集成位置。

### 2.4 协议与代码生成

1. 新增或修改 HTTP/gRPC 接口时，先修改 `api/<业务>/*.proto`，再执行 `make api`；禁止手写或直接编辑生成的 Controller、路由注册、PB、校验和 gRPC 文件。
2. 修改配置结构或领域枚举时，先修改 `internal/conf/conf.proto` 或 `internal/domain/**/meta/*.proto`，再执行 `make config`。
3. 新增构造函数或依赖时，将其加入职责所属层的 ProviderSet，再执行 `make gen` 更新 Wire 生成代码；不得手工修改 `wire_gen.go`。
4. 生成后必须检查 diff，确认生成范围与源文件变更一致，没有意外覆盖手写代码或生成无关文件。

### 2.5 Go 代码规范

1. Go 代码必须通过 `gofmt` 格式化，import 由标准工具整理，不手工制造特殊对齐。
2. 包职责应单一，并与上述 Leo 分层保持一致，不得为少量代码随意新增层级、通用封装或透传接口。
3. 优先传递已有 `context.Context`，不得无故使用 `context.Background()` 切断请求、任务或消息处理链路；仅允许在进程根入口或独立初始化根上下文中创建背景上下文。
4. 错误必须被处理或明确返回；禁止静默吞错。包装错误时保留原始错误链，优先使用 `%w`，并由既有 Leo 协议处理链完成最终错误响应。
5. 资源申请与释放必须成对出现；数据库事务、文件、网络连接、锁和 goroutine 均需考虑异常路径，客户端清理函数应交给 Wire 和进程生命周期统一管理。
6. 涉及并发时必须明确共享状态、退出条件和生命周期；能由 Leo Runner、Stream 或 Server 管理的并发不得自行创建，避免 goroutine 泄漏、重复关闭 channel 和数据竞争。
7. 不使用无业务含义的魔法数字或魔法字符串；稳定且复用的值应定义为常量，状态值应使用语义清晰的常量或类型。
8. 函数保持职责单一。参数过多、分支过深或流程过长时，应优先拆分函数或引入明确的请求、领域命令或配置结构体。
9. 命名应表达业务含义，避免 `data`、`info`、`temp`、`obj` 等过度泛化名称；短作用域变量可遵循 Go 惯例。
10. 保持零值可用原则；确需构造函数时，应在构造阶段完成必要依赖校验，并通过 Wire 注入调用。

## 3. 目录与包命名规范

### 3.1 通用要求

1. 目录名和 Go 包名使用简短、清晰、全小写的英文单词。
2. Go 包目录原则上不使用下划线、连字符、空格、拼音或无意义数字。
3. 名称应优先体现业务职责，不重复父目录已经表达的语义。
4. 可以缩写过长且业界已有广泛共识的名称，但不得自造缩写，也不得为了追求短而牺牲可读性。
5. 同一概念在项目中必须使用同一种命名。创建新目录时应先搜索仓库已有表达，避免产生同义目录。
6. 不因本规范主动批量重命名现有目录；只有任务明确涉及迁移，且调用方、import、文档和测试能够同步更新时才可改名。

### 3.2 推荐缩写

以下缩写可用于新目录、包名或局部变量，前提是语义明确：

| 完整名称 | 推荐缩写 | 示例 |
| --- | --- | --- |
| `repository` | `repo` | `internal/domain/article/repo` |
| `application` | `app` | `internal/app/service` |
| `configuration` | `conf` | 沿用脚手架包名 `internal/conf`；局部变量可使用 `config` 或 `cfg` |
| `database` | `db` | 局部变量 `db`；目录已有 `database` 时不必强改 |
| `authentication` | `auth` | `internal/auth` |
| `authorization` | `authz` | 仅在需要与认证 `auth` 明确区分时使用 |
| `identifier` | `id` | `userID` |
| `request` | `req` | 局部变量 `req` |
| `response` | `resp` | 局部变量 `resp` |
| `remote procedure call` | `rpc` / `grpc` | 按实际协议使用 |

### 3.3 不应缩写的名称

1. `service` 已足够简短且含义明确，目录名和包名必须保留为 `service`，禁止缩写为 `svc`；`server` 目录名和包名禁止缩写为 `srv`，短作用域接收者变量仍可遵循 Go 惯例。
2. `server`、`domain`、`entity`、`consumer`、`command`、`query`、`cache` 等短且清晰的名称通常不缩写。
3. 禁止使用团队外难以理解的自造缩写，例如 `repository -> rpy`、`service -> sv`、`application -> apl`。
4. 缩写发生歧义时使用完整名称；脚手架既有的 `internal/conf` 必须保持一致，短生命周期配置变量可酌情使用 `cfg`。

## 4. 注释撰写规范

以下规范适用于全部手写 Go 代码。自动生成代码不适用，且不得为了补注释直接修改生成文件。

### 4.1 通用要求

1. 注释优先使用中文，表达应简洁、准确、易懂。
2. 注释应说明业务含义、处理目的、关键约束或设计原因，不能只重复代码字面含义。
3. 注释应紧挨被说明的代码，并随代码逻辑同步维护；修改逻辑后必须检查相关注释是否仍准确。
4. 专有名词、错误信息、配置项、数据库字段和接口字段应与代码中的实际名称一致。
5. 不添加“定义变量”“调用函数”“返回结果”等没有额外信息的注释。
6. 较长说明应拆成多行，但要保持语义完整；不得依靠手工空格进行注释对齐。
7. `TODO`、`FIXME` 必须说明待处理事项和原因；能关联任务编号时应一并写明，禁止只写裸 `TODO`。

### 4.2 结构体及字段注释

1. 每个手写结构体的每个字段都必须添加注释，包括匿名字段和嵌入字段。
2. 字段注释统一写在字段定义行末尾，以 `//` 开始，并说明字段的业务含义。
3. 状态、类型、枚举字段必须说明各取值含义；时间字段必须说明具体表示的时间；带单位的数值必须写明单位。
4. 持久化 PO 字段应优先描述业务含义，不能只照抄 XORM 标签或数据库列名。
5. 对外导出的结构体应补充以类型名开头的整体用途注释，符合 Go 文档注释习惯。

```go
// ArticleLike 表示文章点赞记录的持久化对象。
type ArticleLike struct {
	ID        uint64 // 唯一标识
	UserID    uint64 // 用户唯一标识
	ArticleID uint64 // 文章唯一标识
	Status    int8   // 点赞状态：1-点赞；2-取消点赞
	CreatedAt int64  // 创建时间，Unix 秒级时间戳
	UpdatedAt int64  // 最后更新时间，Unix 秒级时间戳
}
```

嵌入字段同样需要说明用途：

```go
type ArticleResponse struct {
	BaseResponse // 通用响应字段
	Article      *Article `json:"article"` // 文章详情
}
```

### 4.3 函数和方法头注释

1. 每个手写的具名函数或方法都应在声明前添加功能注释。
2. 注释应简要说明函数“做什么”，优先使用动宾结构，不展开复述全部实现步骤。
3. 对外导出的函数或方法，注释必须以函数名或方法名开头。
4. 未导出的函数也应添加清晰的功能说明；复杂匿名函数应在代码块前说明用途。
5. 函数头注释与函数声明之间不得插入无关内容。

```go
// GetImageUploadURL 获取文章图片上传凭证。
func (s *ArticleService) GetImageUploadURL(
	ctx *gin.Context,
	req *article.GetImageUploadURLRequest,
) (*article.GetImageUploadURLReply, error) {
	// ...
}
```

### 4.4 函数体流程注释

1. 函数体内按实际执行顺序使用 `1.`、`2.`、`3.` 等一级编号描述主要业务阶段。
2. 一个阶段若包含控制流过滤、协议解析、复杂校验等核心细节，应主动使用 `1.1`、`1.2` 等二级编号分段说明。二级注释应聚焦于有分支、循环或数据转换的关键代码块，避免为纯赋值、常规调用等浅显代码做机械注释。
3. 编号最多只能到二级，禁止出现 `1.1.1` 或更深层级。
4. 流程注释放在对应代码块之前；一个编号应对应一个完整动作，不得机械地为每行代码编号。
5. 同一层级编号应连续。函数仅包含一个简单动作时可只写 `// 1. ...`，不得为了凑步骤虚构流程。
6. 权限校验、幂等、事务边界、缓存失效、软删除、兼容旧数据、资源释放等不易直接看出的关键行为，应补充原因或约束。
7. 普通、直观的错误返回无需逐个重复说明；只有失败处理会改变业务语义时才补充注释。

```go
// GetImageUploadURL 获取文章图片上传凭证。
func (s *ArticleService) GetImageUploadURL(
	ctx *gin.Context,
	req *article.GetImageUploadURLRequest,
) (*article.GetImageUploadURLReply, error) {
	// 1. 调用领域模块生成上传凭证
	uploadURL, fileURL, err := s.image.CreateUploadURL(ctx.Request.Context(), req.FileExt)
	if err != nil {
		return nil, fmt.Errorf("生成文章图片上传凭证: %w", err)
	}

	// 2. 组装 Proto 响应，编码和渲染交给生成代码
	return &article.GetImageUploadURLReply{
		UploadUrl: uploadURL,
		FileUrl:   fileURL,
	}, nil
}
```

复杂初始化流程可采用两级编号：

```go
// Handle 处理文章发布消息。
func (c *ArticleConsumer) Handle(ctx context.Context, msg *stream.Message) error {
	// 1. 解析并校验消息
	// 1.1 将消息负载解析为领域命令
	// 1.2 校验幂等键和必要字段

	// 2. 调用领域模块发布文章

	// 3. 返回处理结果，由 Leo Stream 决定确认或重试
	return nil
}
```

### 4.5 参数较多时的注释

1. 函数或方法的传入参数数量大于等于 5 个时，必须在功能注释后增加“参数说明”。接收者不计入参数数量。
2. 每个参数单独占一行，顺序与函数声明一致，并写明含义及必要约束。
3. 返回值含义不直观或存在特殊约束时再补充返回值说明。
4. 如果参数过多导致接口难以理解，应优先考虑请求结构体或配置结构体，参数注释不能替代合理设计。

```go
// CreateArticle 创建文章。
//
// 参数说明：
//   - ctx：请求上下文，用于传递链路信息和控制超时。
//   - authorID：作者唯一标识。
//   - title：文章标题，不能为空。
//   - content：文章正文内容。
//   - coverURL：文章封面地址，可以为空。
//   - status：文章状态：1-草稿；2-已发布。
func (s *ArticleService) CreateArticle(
	ctx context.Context,
	authorID uint64,
	title string,
	content string,
	coverURL string,
	status int8,
) error {
	// 1. 校验文章参数
	// 2. 组装文章数据
	// 3. 保存文章
	return nil
}
```

### 4.6 接口、常量和包注释

1. 对外提供能力的包建议添加包注释，说明包的职责和边界，并以包名开头。
2. 接口应说明整体职责，接口中的每个方法仍需添加方法注释。
3. 每个对外暴露的常量都应说明含义；枚举或状态常量应明确其业务状态。

```go
// ArticleRepo 定义文章领域所需的数据访问能力。
type ArticleRepo interface {
	// FindByID 根据唯一标识查询文章。
	FindByID(ctx context.Context, id uint64) (*Article, error)
}

const (
	ArticleStatusDraft     int8 = 1 // 草稿
	ArticleStatusPublished int8 = 2 // 已发布
)
```

### 4.7 禁止的注释方式

```go
// 错误：只重复代码动作
// 定义 req
var req Request

// 错误：只写函数名
// CreateArticle
func CreateArticle() {}

// 错误：编号超过二级
// 1.1.1 初始化连接池
```

应改为说明完整业务动作及其目的：

```go
// 1. 解析并校验创建文章请求
var req CreateArticleRequest
```

## 5. 测试与验证规范

1. 修改业务逻辑、错误处理、数据访问或协议转换时，应新增或更新相应测试。
2. 测试至少覆盖本次变更的正常路径、关键失败路径和边界条件；修复缺陷时应优先添加能复现缺陷的回归测试。
3. 测试应可重复执行，不依赖执行顺序、真实生产数据或不可控外部服务。
4. 测试名称应清晰表达被测场景和预期结果；表驱动测试的 case 名称应具备可读性。
5. 完成修改后至少执行：

```bash
gofmt -w <本次修改的 Go 文件>
go test ./...
```

6. 若全量测试因既有环境依赖无法执行，应运行受影响包的测试，并在交付说明中写明未执行项、原因和风险，不得声称测试已全部通过。
7. 修改 `.proto`、ProviderSet 或 `wire.go` 时，必须执行对应的 `make api`、`make config` 或 `make gen`，并验证生成代码能够编译。
8. 测试应用层时应通过领域接口注入 Fake 或 Mock；测试领域层时应围绕领域接口验证规则；测试仓储 Adapter 时不得要求应用服务参与，避免跨层搭建测试环境。

## 6. AI 工作流程

1. **理解任务**：阅读相关代码、测试、文档和配置，列出实际影响范围。
2. **制定方案**：先确认 Leo、生成代码和项目现有 Adapter 是否已有所需能力，再选择最小、清晰、可验证的实现，不做超出需求的架构扩张。
3. **实施修改**：遵守 Leo 分层、能力复用、命名、注释和错误处理规范，并同步维护相关 Proto、ProviderSet、测试和文档。
4. **自检变更**：检查 diff，确认没有调试代码、敏感信息、无关格式化或意外生成文件。
5. **执行验证**：运行格式化、测试及任务所需的构建/静态检查。
6. **交付说明**：明确说明修改内容、关键设计、验证结果以及仍存在的限制或风险。

## 7. 提交前检查清单

- [ ] 修改范围是否仅包含当前任务所需内容？
- [ ] 新代码是否位于 `api`、`app`、`domain`、`repo`、`clients`、`server`、`cmd` 中职责正确的位置？
- [ ] 是否使用贫血领域模型，实体仅保存数据，所有业务规则和状态流转均位于领域服务？
- [ ] 是否避免在实体、PO、Factory 或应用层中实现或复制业务规则？
- [ ] 依赖方向是否保持单向，应用层是否避免直接访问 MySQL、Redis、Kafka 等外部资源？
- [ ] 是否优先复用了 Leo 的生命周期、传输、日志、中间件、配置、Actuator、Stream 和协议处理能力？
- [ ] 是否避免手工重复实现 Proto 生成代码已提供的路由、参数绑定、校验、错误映射和响应渲染？
- [ ] 新增依赖是否通过构造函数、ProviderSet 和 Wire 注入，而不是在业务方法中创建？
- [ ] 目录和包名是否简短、清晰，并只使用公认缩写？
- [ ] 是否沿用 `repo`、`app`、`conf` 等脚手架命名，并保留 `service`、`server` 全称？
- [ ] 是否只修改生成源文件，并通过 `make api`、`make config` 或 `make gen` 更新生成代码？
- [ ] 每个结构体字段是否都有准确的行尾注释？
- [ ] 状态、枚举、时间和带单位字段是否说明清楚？
- [ ] 每个函数或方法是否有功能注释？导出注释是否以标识符开头？
- [ ] 函数体流程注释是否按 `1.`、`2.`、`3.` 编号，且最多使用二级编号？
- [ ] 参数数量大于等于 5 个时，是否逐行补充参数说明？
- [ ] 注释是否解释业务含义和关键约束，而非重复代码？
- [ ] 错误、事务、资源和并发处理是否完整？
- [ ] 是否已执行 `gofmt` 和相关测试？
- [ ] 交付说明是否如实记录验证结果及未解决风险？
