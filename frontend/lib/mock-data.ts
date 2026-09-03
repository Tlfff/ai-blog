// Mock 数据 —— 后端接入后可整体移除
import type { Article, Comment, Notification, Tag, User } from "@/types"

export const tags: Tag[] = [
  { id: "t1", name: "前端", count: 128 },
  { id: "t2", name: "后端", count: 96 },
  { id: "t3", name: "Go", count: 54 },
  { id: "t4", name: "React", count: 87 },
  { id: "t5", name: "TypeScript", count: 73 },
  { id: "t6", name: "架构设计", count: 41 },
  { id: "t7", name: "数据库", count: 38 },
  { id: "t8", name: "DevOps", count: 29 },
  { id: "t9", name: "算法", count: 62 },
  { id: "t10", name: "职业成长", count: 33 },
]

export const users: User[] = [
  {
    id: "u1",
    username: "陈可可",
    avatar: "/avatars/user-chen.png",
    role: "user",
    location: "浙江",
    joinedAt: "2023-03-12",
  },
  {
    id: "u2",
    username: "林深",
    avatar: "/avatars/user-lin.png",
    role: "user",
    location: "北京",
    joinedAt: "2022-11-05",
  },
  {
    id: "u3",
    username: "苏晚",
    avatar: "/avatars/user-su.png",
    role: "user",
    location: "广东",
    joinedAt: "2024-01-20",
  },
  {
    id: "u4",
    username: "阿泽",
    avatar: "/avatars/user-ze.png",
    role: "user",
    location: "上海",
    joinedAt: "2023-07-18",
  },
  {
    id: "admin",
    username: "站长",
    avatar: "/avatars/admin.png",
    role: "admin",
    location: "内网",
    joinedAt: "2021-01-01",
  },
]

function findTags(...names: string[]): Tag[] {
  return tags.filter((t) => names.includes(t.name))
}

const markdownSample = `## 前言

在构建现代博客系统时，我们需要兼顾**性能**、**可维护性**与**用户体验**。本文将从架构层面聊聊我的一些实践。

## 技术选型

- 后端使用 Go，配合 Gin 框架
- 前端使用 Next.js + TypeScript
- 数据库选择 PostgreSQL

\`\`\`go
func main() {
    r := gin.Default()
    r.GET("/ping", func(c *gin.Context) {
        c.JSON(200, gin.H{"message": "pong"})
    })
    r.Run()
}
\`\`\`

## 核心设计

> 好的架构不是设计出来的，而是演进出来的。

在实际开发中，我们通过分层架构将业务逻辑与数据访问解耦，使系统更易于测试和扩展。

## 总结

技术是手段，解决问题才是目的。希望这篇文章能给你带来一些启发。
`

export const articles: Article[] = [
  {
    id: "a1",
    title: "从零构建一个高性能 Go 博客后端",
    summary: "分享我在使用 Go 构建博客后端时的架构思考、分层设计与性能优化经验。",
    content: markdownSample,
    cover: "/go-programming-blog-cover.png",
    author: users[0],
    tags: findTags("Go", "后端", "架构设计"),
    status: "published",
    views: 12840,
    likes: 486,
    commentsCount: 32,
    liked: false,
    createdAt: "2025-07-10T09:20:00Z",
    updatedAt: "2025-07-10T09:20:00Z",
  },
  {
    id: "a2",
    title: "React Server Components 深度解析",
    summary: "RSC 到底解决了什么问题？本文带你彻底理解服务端组件的渲染模型。",
    content: markdownSample,
    cover: "/react-server-components.png",
    author: users[1],
    tags: findTags("React", "前端", "TypeScript"),
    status: "published",
    views: 9820,
    likes: 372,
    commentsCount: 21,
    liked: true,
    createdAt: "2025-07-08T14:00:00Z",
    updatedAt: "2025-07-09T10:00:00Z",
  },
  {
    id: "a3",
    title: "TypeScript 类型体操：从入门到放弃再到精通",
    summary: "深入 TypeScript 高级类型，掌握条件类型、映射类型与模板字面量类型。",
    cover: "/typescript-code-abstract.png",
    content: markdownSample,
    author: users[2],
    tags: findTags("TypeScript", "前端"),
    status: "published",
    views: 7650,
    likes: 298,
    commentsCount: 18,
    createdAt: "2025-07-05T11:30:00Z",
    updatedAt: "2025-07-05T11:30:00Z",
  },
  {
    id: "a4",
    title: "PostgreSQL 索引优化实战",
    summary: "慢查询频发？本文教你如何通过索引设计将查询性能提升 10 倍。",
    cover: "/database-optimization-concept.png",
    content: markdownSample,
    author: users[3],
    tags: findTags("数据库", "后端"),
    status: "published",
    views: 6420,
    likes: 254,
    commentsCount: 15,
    createdAt: "2025-07-02T16:45:00Z",
    updatedAt: "2025-07-02T16:45:00Z",
  },
  {
    id: "a5",
    title: "前端工程化：从零搭建现代化构建体系",
    summary: "Monorepo、模块联邦、CI/CD，一文讲透前端工程化的方方面面。",
    cover: "/frontend-engineering-workflow.png",
    content: markdownSample,
    author: users[1],
    tags: findTags("前端", "DevOps"),
    status: "published",
    views: 5310,
    likes: 189,
    commentsCount: 12,
    createdAt: "2025-06-28T08:15:00Z",
    updatedAt: "2025-06-28T08:15:00Z",
  },
  {
    id: "a6",
    title: "分布式系统中的一致性算法：Raft 详解",
    summary: "用最通俗的语言讲清楚 Raft 共识算法的选举、日志复制与安全性。",
    cover: "/distributed-systems-network.png",
    content: markdownSample,
    author: users[0],
    tags: findTags("架构设计", "算法", "后端"),
    status: "published",
    views: 4980,
    likes: 176,
    commentsCount: 9,
    createdAt: "2025-06-25T13:00:00Z",
    updatedAt: "2025-06-25T13:00:00Z",
  },
  {
    id: "a7",
    title: "我的三年后端进阶之路",
    summary: "从初级到资深，聊聊这三年我踩过的坑和积累的经验。",
    cover: "/career-growth-path.png",
    content: markdownSample,
    author: users[2],
    tags: findTags("职业成长", "后端"),
    status: "published",
    views: 8730,
    likes: 421,
    commentsCount: 27,
    createdAt: "2025-06-20T10:00:00Z",
    updatedAt: "2025-06-20T10:00:00Z",
  },
  {
    id: "a8",
    title: "（草稿）Kubernetes 生产环境最佳实践",
    summary: "整理中的一篇关于 K8s 生产部署的长文。",
    content: markdownSample,
    author: users[0],
    tags: findTags("DevOps", "架构设计"),
    status: "draft",
    views: 0,
    likes: 0,
    commentsCount: 0,
    createdAt: "2025-07-15T09:00:00Z",
    updatedAt: "2025-07-18T09:00:00Z",
  },
]

export const comments: Comment[] = [
  {
    id: "c1",
    articleId: "a1",
    parentId: null,
    author: users[1],
    content: "写得非常好！分层架构那部分对我启发很大，请问服务层和仓储层之间你是怎么处理事务的？",
    likes: 24,
    liked: false,
    createdAt: "2025-07-10T10:30:00Z",
    replyCount: 2,
    replies: [
      {
        id: "c1r1",
        articleId: "a1",
        parentId: "c1",
        replyToUser: { id: "u2", username: "林深" },
        author: users[0],
        content: "我一般把事务放在服务层，通过 context 传递事务对象给仓储层。",
        likes: 8,
        createdAt: "2025-07-10T11:00:00Z",
      },
      {
        id: "c1r2",
        articleId: "a1",
        parentId: "c1",
        replyToUser: { id: "u1", username: "陈可可" },
        author: users[2],
        content: "学到了，感谢分享！",
        likes: 3,
        createdAt: "2025-07-10T11:20:00Z",
      },
    ],
  },
  {
    id: "c2",
    articleId: "a1",
    parentId: null,
    author: users[3],
    content: "Go 的错误处理确实需要一些心智负担，期待作者能出一篇专门讲错误处理的文章。",
    likes: 15,
    liked: true,
    createdAt: "2025-07-10T12:00:00Z",
    replyCount: 0,
    replies: [],
  },
  {
    id: "c3",
    articleId: "a1",
    parentId: null,
    author: users[2],
    content: "这条评论已被删除。",
    deleted: true,
    likes: 0,
    createdAt: "2025-07-10T13:00:00Z",
    replyCount: 1,
    replies: [
      {
        id: "c3r1",
        articleId: "a1",
        parentId: "c3",
        author: users[3],
        content: "虽然楼主删了，但我的回复还在，这就是保留评论树的效果。",
        likes: 5,
        createdAt: "2025-07-10T13:30:00Z",
      },
    ],
  },
]

export const notifications: Notification[] = [
  {
    id: "n1",
    type: "article_like",
    actor: users[1],
    read: false,
    createdAt: "2025-07-20T09:00:00Z",
    articleId: "a1",
    articleTitle: "从零构建一个高性能 Go 博客后端",
  },
  {
    id: "n2",
    type: "comment_reply",
    actor: users[2],
    read: false,
    createdAt: "2025-07-19T18:30:00Z",
    articleId: "a1",
    actionText: "回复了你的评论",
  },
  {
    id: "n3",
    type: "comment_like",
    actor: users[3],
    read: true,
    createdAt: "2025-07-18T14:10:00Z",
    articleId: "a1",
    actionText: "点赞了你的评论",
  },
]

// 当前登录用户（mock）。设为 null 即为游客视角。
export const currentUserMock: User = users[0]
export const adminUserMock: User = users[4]
