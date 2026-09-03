import type { Article, Comment, CommentSort, Paginated } from "@/types"
import { mapBackendRootCommentToFrontend, mapBackendReplyCommentToFrontend } from "@/types"
import { getAdminArticleList } from "./articles"
import { request } from "./client"

export interface CommentQuery {
  articleId: string
  page?: number
  pageSize?: number
  sort?: CommentSort
  authorOnlyId?: string
}

export interface CreateCommentRequest {
  articleId: string
  content: string
  rootId?: number
  replyToUserId?: number
}

export interface AdminCommentCollection {
  articles: Article[]
  comments: Comment[]
}

export async function getComments(query: CommentQuery): Promise<Paginated<Comment>> {
  const { articleId, page = 1, pageSize = 10, sort = "newest", authorOnlyId } = query

  const params = new URLSearchParams()
  params.set("article_id", articleId)
  params.set("page", String(page))
  
  // Enforce backend validation constraint: page_size must be min=10, max=20
  const limitedPageSize = Math.max(10, Math.min(20, pageSize))
  params.set("page_size", String(limitedPageSize))
  params.set("is_desc", String(sort === "newest"))
  if (authorOnlyId) params.set("author_id", authorOnlyId)

  const data = await request<{
    list: {
      id: number
      article_id: number
      user: { user_id: number; username: string; avatar: string }
      content: string
      reply_count: number
      ip: string
      created_time: number
      status: number
      like_count: number
    }[]
    total: number
    last_id: number
    page: number
    page_size: number
  }>(`/comment/list/roots?${params.toString()}`)

  return {
    items: data.list.map((item) => mapBackendRootCommentToFrontend(item as any)),
    total: data.total,
    page: data.page,
    pageSize: data.page_size,
  }
}

async function getReplyPage(rootId: string, page = 1, pageSize = 20): Promise<Paginated<Comment>> {
  const params = new URLSearchParams()
  params.set("root_id", rootId)
  params.set("page", String(page))
  
  // Enforce backend validation constraint: page_size must be min=10, max=20
  const limitedPageSize = Math.max(10, Math.min(20, pageSize))
  params.set("page_size", String(limitedPageSize))

  const data = await request<{
    list: {
      id: number
      article_id: number
      root_id: number
      user: { user_id: number; username: string; avatar: string }
      reply_to_user?: { user_id: number; username: string; avatar: string }
      content: string
      created_time: number
      status: number
      ip: string
      like_count: number
    }[]
    total: number
    last_id: number
    page: number
    page_size: number
  }>(`/comment/list/replies?${params.toString()}`)

  return {
    items: data.list.map((item) => mapBackendReplyCommentToFrontend(item as any)),
    total: data.total,
    page: data.page,
    pageSize: data.page_size,
  }
}

export async function getReplies(rootId: string): Promise<Comment[]> {
  const firstPage = await getReplyPage(rootId)
  const replies = [...firstPage.items]
  const totalPages = Math.ceil(firstPage.total / firstPage.pageSize)

  if (totalPages > 1) {
    const remainingPages = await Promise.all(
      Array.from({ length: totalPages - 1 }, (_, index) => getReplyPage(rootId, index + 2)),
    )
    remainingPages.forEach((page) => replies.push(...page.items))
  }

  return replies
}

async function getAllAdminArticles(): Promise<Article[]> {
  const firstPage = await getAdminArticleList(-2, 1, 20)
  const articles = [...firstPage.items]
  const totalPages = Math.ceil(firstPage.total / firstPage.pageSize)

  for (let page = 2; page <= totalPages; page += 1) {
    const result = await getAdminArticleList(-2, page, 20)
    articles.push(...result.items)
  }

  return articles
}

async function getAllCommentsForArticle(articleId: string): Promise<Comment[]> {
  const firstPage = await getComments({ articleId, page: 1, pageSize: 20 })
  const roots = [...firstPage.items]
  const totalPages = Math.ceil(firstPage.total / firstPage.pageSize)

  for (let page = 2; page <= totalPages; page += 1) {
    const result = await getComments({ articleId, page, pageSize: 20 })
    roots.push(...result.items)
  }

  const replies = await Promise.all(
    roots
      .filter((comment) => (comment.replyCount ?? 0) > 0)
      .map((comment) => getReplies(comment.id)),
  )

  return [...roots, ...replies.flat()]
}

export async function getAdminCommentCollection(): Promise<AdminCommentCollection> {
  const articles = await getAllAdminArticles()
  const commentResults = await Promise.allSettled(
    articles.map((article) => getAllCommentsForArticle(article.id)),
  )
  const comments = commentResults.flatMap((result) =>
    result.status === "fulfilled" ? result.value : [],
  )

  return { articles, comments }
}

export async function createComment(input: CreateCommentRequest): Promise<Comment> {
  const data = await request<{ id: number; created_time: number }>("/auth/comment/create", {
    method: "POST",
    body: JSON.stringify({
      article_id: Number(input.articleId),
      root_id: input.rootId || 0,
      reply_to_user_id: input.replyToUserId || 0,
      content: input.content,
    }),
  })

  return {
    id: String(data.id),
    articleId: input.articleId,
    parentId: input.rootId ? String(input.rootId) : null,
    author: {
      id: "",
      username: "",
      avatar: "",
      role: "user",
      location: "",
      joinedAt: new Date(data.created_time * 1000).toISOString(),
    },
    content: input.content,
    likes: 0,
    createdAt: new Date(data.created_time * 1000).toISOString(),
    replies: [],
    replyCount: 0,
  }
}

export async function likeComment(commentId: string): Promise<void> {
  await request<void>("/auth/comment/like", {
    method: "POST",
    body: JSON.stringify({ comment_id: Number(commentId) }),
  })
}

export async function cancelLikeComment(commentId: string): Promise<void> {
  await request<void>("/auth/comment/cancel_like", {
    method: "POST",
    body: JSON.stringify({ comment_id: Number(commentId) }),
  })
}

export async function toggleCommentLike(id: string, liked: boolean): Promise<void> {
  if (liked) {
    await cancelLikeComment(id)
  } else {
    await likeComment(id)
  }
}

export async function deleteComment(id: string): Promise<void> {
  await request<void>("/auth/comment/delete", {
    method: "POST",
    body: JSON.stringify({ id: Number(id) }),
  })
}

export async function adminDeleteComment(id: string): Promise<void> {
  await request<void>("/admin/comment/delete", {
    method: "POST",
    body: JSON.stringify({ id: Number(id) }),
  })
}
