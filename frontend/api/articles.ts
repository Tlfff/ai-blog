import type { Article, Paginated, Tag } from "@/types"
import { mapBackendArticleDetailToFrontend } from "@/types"
import { request, saveToLocalHistory } from "./client"
import { getPublicProfile } from "./users"

export interface ArticleSearchQuery {
  keyword: string
  page?: number
  pageSize?: number
}

export interface ArticleSearchItem {
  id: number
  title: string
  titleHighlight: string
  summary: string
  tags: string[]
}

export interface ArticleSearchResult {
  items: ArticleSearchItem[]
  total: number
  page: number
  pageSize: number
}

export interface ArticleQuery {
  page?: number
  pageSize?: number
  tag?: string
  authorId?: string
  status?: Article["status"]
  keyword?: string
  isDesc?: boolean
}

export interface CreateArticleRequest {
  title: string
  content: string
  tags?: string[]
  status?: number
}

export interface UpdateArticleRequest extends CreateArticleRequest {
  id: number
}

export interface ArticleImageUploadCredential {
  image_id: number
  upload_url: string
  url: string
}

export interface DeleteArticleRequest {
  id: number
}

const authorCache = new Map<string, { username: string; avatar: string }>()

export async function getArticleImageUploadURL(fileExt: string): Promise<ArticleImageUploadCredential> {
  return request<ArticleImageUploadCredential>("/admin/article/image/upload-url", {
    method: "POST",
    body: JSON.stringify({ file_ext: fileExt }),
  })
}

export async function uploadArticleImage(file: File, uploadURL: string): Promise<void> {
  const uploadResponse = await fetch(uploadURL, {
    method: "PUT",
    body: file,
    headers: {
      "Content-Type": file.type || "application/octet-stream",
    },
  })

  if (!uploadResponse.ok) {
    throw new Error("图片上传到存储服务失败")
  }
}

export async function createArticle(data: CreateArticleRequest): Promise<string> {
  const status = data.status === 3 ? 3 : 2
  await request<void>("/admin/article/create", {
    method: "POST",
    body: JSON.stringify({
      title: data.title,
      content: data.content,
      tags: data.tags || [],
      status,
    }),
  })

  const articles = await getAdminArticleList(status, 1, 10)
  const createdArticle = articles.items[0]
  if (!createdArticle) {
    throw new Error("文章创建成功，但未能读取新文章")
  }
  return createdArticle.id
}

export async function getAuthorInfo(authorId: string) {
  if (!authorId) return { username: "未知用户", avatar: "" }
  if (authorCache.has(authorId)) {
    return authorCache.get(authorId)!
  }
  try {
    const profile = await getPublicProfile(authorId)
    const info = { username: profile.username, avatar: profile.avatar }
    authorCache.set(authorId, info)
    return info
  } catch {
    const fallback = { username: "睦子米", avatar: "" }
    authorCache.set(authorId, fallback)
    return fallback
  }
}

export async function searchArticles(query: ArticleSearchQuery): Promise<ArticleSearchResult> {
  const keyword = query.keyword.trim()
  const page = Math.max(1, query.page ?? 1)
  const pageSize = Math.max(10, Math.min(20, query.pageSize ?? 10))

  const params = new URLSearchParams({
    keyword,
    page: String(page),
    page_size: String(pageSize),
  })

  const data = await request<{
    list: {
      id: number
      title: string
      title_highlight: string
      summary: string
      tags: string[]
    }[]
    total: number
    page: number
    page_size: number
  }>(`/article/search?${params.toString()}`)

  return {
    items: data.list.map((item) => ({
      id: item.id,
      title: item.title,
      titleHighlight: item.title_highlight,
      summary: item.summary,
      tags: item.tags ?? [],
    })),
    total: data.total,
    page: data.page,
    pageSize: data.page_size,
  }
}

export async function getArticles(query: ArticleQuery = {}): Promise<Paginated<Article>> {
  const { page = 1, pageSize = 10, keyword, isDesc = true } = query

  const params = new URLSearchParams()
  params.set("page", String(page))
  
  // Enforce backend validation constraint: page_size must be min=10, max=20
  const limitedPageSize = Math.max(10, Math.min(20, keyword ? 20 : pageSize))
  params.set("page_size", String(limitedPageSize))
  params.set("is_desc", String(isDesc))

  const data = await request<{
    list: { id: number; title: string; summary: string; author_id: number; updated_time: number; view_count: number; like_count: number; comment_count: number }[]
    total: number
    page: number
    page_size: number
    last_id: number
  }>(`/article/list?${params.toString()}`)

  let list = data.list
  if (keyword) {
    const kw = keyword.toLowerCase()
    list = list.filter((item) =>
      item.title.toLowerCase().includes(kw) ||
      item.summary.toLowerCase().includes(kw)
    )
  }

  const items = await Promise.all(
    list.map(async (item) => {
      const authorInfo = await getAuthorInfo(String(item.author_id))
      return {
        id: String(item.id),
        title: item.title,
        summary: item.summary,
        content: "",
        author: {
          id: String(item.author_id),
          username: authorInfo.username,
          avatar: authorInfo.avatar,
          role: "user" as const,
          location: "",
          joinedAt: new Date(item.updated_time * 1000).toISOString(),
        },
        tags: [],
        status: "published" as const,
        views: item.view_count,
        likes: item.like_count,
        commentsCount: item.comment_count,
        createdAt: new Date(item.updated_time * 1000).toISOString(),
        updatedAt: new Date(item.updated_time * 1000).toISOString(),
      }
    })
  )

  const paginatedItems = keyword
    ? items.slice((page - 1) * pageSize, page * pageSize)
    : items

  return {
    items: paginatedItems,
    total: keyword ? items.length : data.total,
    page,
    pageSize,
  }
}

interface PublishedArticleMetadata {
  authorId: string
  viewCount: number
  likeCount: number
  commentCount: number
}

async function getPublishedArticleMetadata(id: string): Promise<PublishedArticleMetadata | undefined> {
  const pageSize = 20
  let page = 1

  while (true) {
    const data = await request<{
      list: {
        id: number
        author_id: number
        view_count: number
        like_count: number
        comment_count: number
      }[]
      total: number
      page_size: number
    }>(`/article/list?page=${page}&page_size=${pageSize}&is_desc=true`)
    const item = data.list.find((article) => String(article.id) === id)

    if (item) {
      return {
        authorId: String(item.author_id),
        viewCount: item.view_count,
        likeCount: item.like_count,
        commentCount: item.comment_count,
      }
    }

    if (page * data.page_size >= data.total) return undefined
    page += 1
  }
}

export async function getArticleById(id: string): Promise<Article | undefined> {
  try {
    const data = await request<{
      id: number
      title: string
      content: string
      tags: string[]
      status: number
      author_nick: string
      author_avatar: string
      ip: string
      created_time: number
      updated_time: number
      is_liked: boolean
      like_count: number
      images: { id: number; url: string }[]
    }>(`/optional/article/detail?id=${id}`)
    
    const metadata = await getPublishedArticleMetadata(id).catch(() => undefined)
    const article = mapBackendArticleDetailToFrontend(data)
    if (metadata) {
      article.author.id = metadata.authorId
      article.views = metadata.viewCount
      article.likes = metadata.likeCount
      article.commentsCount = metadata.commentCount
    }

    saveToLocalHistory(String(data.id), data.title)
    return article
  } catch {
  }
  try {
    const data = await request<{
      id: number
      title: string
      content: string
      tags: string[]
      status: number
      author_nick: string
      author_avatar: string
      ip: string
      created_time: number
      updated_time: number
      is_liked: boolean
      like_count: number
      images: { id: number; url: string }[]
    }>(`/admin/article/me/detail?id=${id}`)
    
    saveToLocalHistory(String(data.id), data.title)
    return mapBackendArticleDetailToFrontend(data)
  } catch {
    return undefined
  }
}

export async function getHotArticles(limit = 10): Promise<Article[]> {
  const data = await request<{
    list: { article_id: number; title: string; hot: number; view_count: number; comment_count: number; like_count: number }[]
  }>("/article/hot-rank")

  return data.list.slice(0, limit).map((item) => ({
    id: String(item.article_id),
    title: item.title,
    summary: "",
    content: "",
    author: {
      id: "",
      username: "",
      avatar: "",
      role: "user",
      location: "",
      joinedAt: new Date().toISOString(),
    },
    tags: [],
    status: "published",
    views: item.view_count,
    likes: item.like_count,
    commentsCount: item.comment_count,
    createdAt: new Date().toISOString(),
    updatedAt: new Date().toISOString(),
  }))
}

export async function getStats(): Promise<{ articles: number; views: number; likes: number }> {
  try {
    const pageSize = 20
    const firstPage = await request<{
      list: { view_count: number; like_count: number }[]
      total: number
      page: number
      page_size: number
    }>(`/article/list?page=1&page_size=${pageSize}&is_desc=true`)

    let totalViews = firstPage.list.reduce((sum, item) => sum + item.view_count, 0)
    let totalLikes = firstPage.list.reduce((sum, item) => sum + item.like_count, 0)

    const totalPages = Math.ceil(firstPage.total / firstPage.page_size)
    if (totalPages > 1) {
      const remainingPages = await Promise.all(
        Array.from({ length: totalPages - 1 }, (_, index) =>
          request<{
            list: { view_count: number; like_count: number }[]
          }>(`/article/list?page=${index + 2}&page_size=${pageSize}&is_desc=true`),
        ),
      )
      remainingPages.forEach((data) => {
        totalViews += data.list.reduce((sum, item) => sum + item.view_count, 0)
        totalLikes += data.list.reduce((sum, item) => sum + item.like_count, 0)
      })
    }

    return {
      articles: firstPage.total,
      views: totalViews,
      likes: totalLikes,
    }
  } catch (error) {
    console.error("getStats error:", error)
    return {
      articles: 0,
      views: 0,
      likes: 0,
    }
  }
}

export async function likeArticle(articleId: string): Promise<void> {
  await request<void>("/auth/article/like", {
    method: "POST",
    body: JSON.stringify({ article_id: Number(articleId) }),
  })
}

export async function cancelLikeArticle(articleId: string): Promise<void> {
  await request<void>("/auth/article/cancel_like", {
    method: "POST",
    body: JSON.stringify({ article_id: Number(articleId) }),
  })
}

export async function toggleArticleLike(id: string): Promise<Article | undefined> {
  const article = await getArticleById(id)
  if (!article) return undefined

  if (article.liked) {
    await cancelLikeArticle(id)
  } else {
    await likeArticle(id)
  }

  return getArticleById(id)
}

export async function updateArticle(id: string, data: Partial<CreateArticleRequest>): Promise<Article | undefined> {
  await request<void>("/admin/article/update", {
    method: "POST",
    body: JSON.stringify({
      id: Number(id),
      title: data.title,
      content: data.content,
      tags: data.tags || [],
      status: data.status,
    }),
  })

  return getArticleById(id)
}

export async function deleteArticle(id: string): Promise<void> {
  await request<void>("/admin/article/delete", {
    method: "POST",
    body: JSON.stringify({ id: Number(id) }),
  })
}

export async function publishArticle(id: string): Promise<void> {
  await request<void>("/admin/article/publish", {
    method: "POST",
    body: JSON.stringify({ id: Number(id) }),
  })
}

export async function recordView(articleId: string): Promise<void> {
  // View recording is handled automatically by GET /optional/article/detail
}

async function getAdminArticlePage(status: number, page = 1, pageSize = 10): Promise<Paginated<Article>> {
  const params = new URLSearchParams()
  params.set("status", String(status))
  params.set("page", String(page))
  const limitedPageSize = Math.max(10, Math.min(20, pageSize))
  params.set("page_size", String(limitedPageSize))
  params.set("is_desc", "true")

  const data = await request<{
    list: { id: number; title: string; tags: string[]; status: number; created_time: number; updated_time: number; view_count?: number; like_count?: number; comment_count?: number }[]
    total: number
    page: number
    page_size: number
    last_id: number
  }>(`/admin/article/list?${params.toString()}`)

  return {
    items: data.list.map((item) => ({
      id: String(item.id),
      title: item.title,
      summary: "",
      content: "",
      author: {
        id: "",
        username: "",
        avatar: "",
        role: "user",
        location: "",
        joinedAt: new Date(item.created_time * 1000).toISOString(),
      },
      tags: item.tags.map((name) => ({ id: name, name, count: 0 })),
      status: item.status === 3 ? "published" : item.status === 2 ? "draft" : "deleted",
      views: item.view_count || 0,
      likes: item.like_count || 0,
      commentsCount: item.comment_count || 0,
      createdAt: new Date(item.created_time * 1000).toISOString(),
      updatedAt: new Date(item.updated_time * 1000).toISOString(),
    })),
    total: data.total,
    page: data.page,
    pageSize: data.page_size,
  }
}

export async function getAdminArticleList(
  status: number,
  page = 1,
  pageSize = 10,
  keyword = "",
): Promise<Paginated<Article>> {
  const normalizedKeyword = keyword.trim().toLowerCase()
  if (!normalizedKeyword) {
    return getAdminArticlePage(status, page, pageSize)
  }

  const firstPage = await getAdminArticlePage(status, 1, 20)
  const articles = [...firstPage.items]
  const totalPages = Math.ceil(firstPage.total / firstPage.pageSize)

  if (totalPages > 1) {
    const remainingPages = await Promise.all(
      Array.from({ length: totalPages - 1 }, (_, index) =>
        getAdminArticlePage(status, index + 2, 20),
      ),
    )
    remainingPages.forEach((result) => articles.push(...result.items))
  }

  const filteredArticles = articles.filter((article) =>
    article.title.toLowerCase().includes(normalizedKeyword),
  )
  const safePageSize = Math.max(10, Math.min(20, pageSize))
  const start = (page - 1) * safePageSize

  return {
    items: filteredArticles.slice(start, start + safePageSize),
    total: filteredArticles.length,
    page,
    pageSize: safePageSize,
  }
}

export async function getTrashList(page = 1, pageSize = 10): Promise<Paginated<Article>> {
  const params = new URLSearchParams()
  params.set("status", "1")
  params.set("page", String(page))
  const limitedPageSize = Math.max(10, Math.min(20, pageSize))
  params.set("page_size", String(limitedPageSize))
  params.set("is_desc", "true")

  const data = await request<{
    list: { id: number; title: string; tags: string[]; status: number; created_time: number; updated_time: number; view_count?: number; like_count?: number; comment_count?: number }[]
    total: number
    page: number
    page_size: number
    last_id: number
  }>(`/admin/article/trash/list?${params.toString()}`)

  return {
    items: data.list.map((item) => ({
      id: String(item.id),
      title: item.title,
      summary: "",
      content: "",
      author: {
        id: "",
        username: "",
        avatar: "",
        role: "user",
        location: "",
        joinedAt: new Date(item.created_time * 1000).toISOString(),
      },
      tags: item.tags.map((name) => ({ id: name, name, count: 0 })),
      status: "deleted",
      views: item.view_count || 0,
      likes: item.like_count || 0,
      commentsCount: item.comment_count || 0,
      createdAt: new Date(item.created_time * 1000).toISOString(),
      updatedAt: new Date(item.updated_time * 1000).toISOString(),
    })),
    total: data.total,
    page: data.page,
    pageSize: data.page_size,
  }
}

export async function recoverArticle(id: string): Promise<void> {
  await request<void>("/admin/article/trash/recover", {
    method: "POST",
    body: JSON.stringify({ id: Number(id), status: 2 }),
  })
}

export async function hardDeleteArticle(id: string): Promise<void> {
  await request<void>("/admin/article/trash/clear", {
    method: "POST",
    body: JSON.stringify({ id: Number(id) }),
  })
}

export async function getTags(): Promise<Tag[]> {
  try {
    const articles = await getArticles({ page: 1, pageSize: 20 })
    const tagMap = new Map<string, number>()
    articles.items.forEach((article) => {
      article.tags.forEach((tag) => {
        const count = tagMap.get(tag.name) || 0
        tagMap.set(tag.name, count + 1)
      })
    })
    const tags: Tag[] = Array.from(tagMap.entries()).map(([name, count]) => ({
      id: name,
      name,
      count,
    }))
    return tags.sort((a, b) => b.count - a.count)
  } catch (error) {
    console.error("getTags error:", error)
    return []
  }
}
