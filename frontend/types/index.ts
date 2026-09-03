export type Role = "guest" | "user" | "admin"

export interface User {
  id: string
  username: string
  avatar: string
  role: Role
  location: string
  lastLoginTime?: number
  lastLoginIp?: string
  joinedAt: string
}

export interface Tag {
  id: string
  name: string
  count: number
}

export type ArticleStatus = "published" | "draft" | "deleted"

export interface Article {
  id: string
  title: string
  summary: string
  content: string
  cover?: string
  author: User
  tags: Tag[]
  status: ArticleStatus
  views: number
  likes: number
  commentsCount: number
  liked?: boolean
  images?: ArticleImage[]
  createdAt: string
  updatedAt: string
}

export interface ArticleImage {
  id: string
  url: string
}

export interface Comment {
  id: string
  articleId: string
  parentId: string | null
  replyToUser?: Pick<User, "id" | "username">
  author: User
  content: string
  likes: number
  liked?: boolean
  createdAt: string
  deleted?: boolean
  replies?: Comment[]
  replyCount?: number
  ip?: string
}

export type NotificationType = "article_like" | "comment_reply" | "comment_like"

export interface Notification {
  id: string
  type: NotificationType
  actor: User
  read: boolean
  createdAt: string
  articleId?: string
  articleTitle?: string
  actionText?: string
}

export type CommentSort = "newest" | "oldest"

export interface HistoryItem {
  articleId: string
  title: string
  viewedAt: string
}

export interface Paginated<T> {
  items: T[]
  total: number
  page: number
  pageSize: number
}

export interface BackendResponse<T = any> {
  success: boolean
  code: number
  message: string
  data: T
}

export interface BackendUser {
  id: number
  nickname: string
  avatar: string
  role: Role
}

export interface BackendMyProfile extends BackendUser {
  last_login_time: number
  last_login_ip: string
}

export interface BackendPublicProfile {
  id: number
  nickname: string
  avatar: string
}

export interface BackendArticleDetail {
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
  view_count?: number
  comment_count?: number
  images?: { id: number; url: string }[]
}

export interface BackendArticleListItem {
  id: number
  title: string
  summary: string
  author_id: number
  updated_time: number
}

export interface BackendArticleListResponse {
  list: BackendArticleListItem[]
  last_id: number
  total: number
  page: number
  page_size: number
}

export interface BackendAdminListItem {
  id: number
  title: string
  tags: string[]
  status: number
  created_time: number
  updated_time: number
}

export interface BackendAdminListResponse {
  list: BackendAdminListItem[]
  last_id: number
  total: number
  page: number
  page_size: number
}

export interface BackendHotRankItem {
  article_id: number
  title: string
  hot: number
  view_count: number
  comment_count: number
  like_count: number
}

export interface BackendHotRankResponse {
  list: BackendHotRankItem[]
}

export interface BackendCommentUserInfo {
  user_id: number
  username: string
  avatar: string
}

export interface BackendRootComment {
  id: number
  article_id: number
  user: BackendCommentUserInfo
  content: string
  reply_count: number
  ip: string
  created_time: number
  status: number
  like_count: number
}

export interface BackendReplyComment {
  id: number
  article_id: number
  root_id: number
  user: BackendCommentUserInfo
  reply_to_user?: BackendCommentUserInfo
  content: string
  created_time: number
  status: number
  ip: string
  like_count: number
}

export interface BackendCommentListResponse {
  list: BackendRootComment[] | BackendReplyComment[]
  total: number
  last_id: number
  page: number
  page_size: number
}

export interface BackendNotification {
  id: string
  type: number
  is_read: boolean
  created_time: number
  sender_id: number
  sender_nickname: string
  sender_avatar: string
  action_text: string
  article_id?: number
  title?: string
}

export interface BackendNotificationListResponse {
  list: BackendNotification[]
  page: number
  page_size: number
}

export function formatAvatarUrl(avatar?: string): string {
  if (!avatar) return ""
  if (avatar.startsWith("http://") || avatar.startsWith("https://") || avatar.startsWith("data:")) {
    return avatar
  }
  return `https://muzimi.xyz:9000/blog-images/${avatar}`
}

export function mapBackendUserToFrontend(user: BackendUser): User {
  return {
    id: String(user.id),
    username: user.nickname,
    avatar: formatAvatarUrl(user.avatar),
    role: user.role,
    location: "",
    joinedAt: new Date().toISOString(),
  }
}

export function mapBackendMyProfileToFrontend(user: BackendMyProfile): User {
  return {
    id: String(user.id),
    username: user.nickname,
    avatar: formatAvatarUrl(user.avatar),
    role: user.role,
    location: user.last_login_ip,
    lastLoginTime: user.last_login_time,
    lastLoginIp: user.last_login_ip,
    joinedAt: new Date(user.last_login_time * 1000).toISOString(),
  }
}

export function mapBackendPublicProfileToFrontend(user: BackendPublicProfile): User {
  return {
    id: String(user.id),
    username: user.nickname,
    avatar: formatAvatarUrl(user.avatar),
    role: "user",
    location: "",
    joinedAt: new Date().toISOString(),
  }
}

export function mapBackendArticleDetailToFrontend(article: BackendArticleDetail): Article {
  return {
    id: String(article.id),
    title: article.title,
    content: article.content,
    summary: article.content.length > 50 ? article.content.slice(0, 50) + "..." : article.content,
    author: {
      id: "",
      username: article.author_nick,
      avatar: formatAvatarUrl(article.author_avatar),
      role: "user",
      location: article.ip,
      joinedAt: new Date(article.created_time * 1000).toISOString(),
    },
    tags: article.tags.map((name) => ({ id: name, name, count: 0 })),
    status: article.status === 3 ? "published" : "draft",
    views: article.view_count || 0,
    likes: article.like_count,
    commentsCount: article.comment_count || 0,
    liked: article.is_liked,
    images: (article.images ?? []).map((image) => ({
      id: String(image.id),
      url: image.url,
    })),
    createdAt: new Date(article.created_time * 1000).toISOString(),
    updatedAt: new Date(article.updated_time * 1000).toISOString(),
  }
}

export function mapBackendArticleListItemToFrontend(item: BackendArticleListItem): Partial<Article> {
  return {
    id: String(item.id),
    title: item.title,
    summary: item.summary,
    createdAt: new Date(item.updated_time * 1000).toISOString(),
    updatedAt: new Date(item.updated_time * 1000).toISOString(),
  }
}

export function mapBackendRootCommentToFrontend(comment: BackendRootComment): Comment {
  return {
    id: String(comment.id),
    articleId: String(comment.article_id),
    parentId: null,
    author: {
      id: String(comment.user.user_id),
      username: comment.user.username,
      avatar: formatAvatarUrl(comment.user.avatar),
      role: "user",
      location: comment.ip,
      joinedAt: new Date(comment.created_time * 1000).toISOString(),
    },
    content: comment.content,
    likes: comment.like_count,
    deleted: comment.status === 0,
    createdAt: new Date(comment.created_time * 1000).toISOString(),
    replyCount: comment.reply_count,
    ip: comment.ip,
  }
}

export function mapBackendReplyCommentToFrontend(comment: BackendReplyComment): Comment {
  return {
    id: String(comment.id),
    articleId: String(comment.article_id),
    parentId: String(comment.root_id),
    author: {
      id: String(comment.user.user_id),
      username: comment.user.username,
      avatar: formatAvatarUrl(comment.user.avatar),
      role: "user",
      location: comment.ip,
      joinedAt: new Date(comment.created_time * 1000).toISOString(),
    },
    replyToUser: comment.reply_to_user
      ? {
          id: String(comment.reply_to_user.user_id),
          username: comment.reply_to_user.username,
        }
      : undefined,
    content: comment.content,
    likes: comment.like_count,
    deleted: comment.status === 0,
    createdAt: new Date(comment.created_time * 1000).toISOString(),
    ip: comment.ip,
  }
}

export function mapBackendNotificationToFrontend(notification: BackendNotification): Notification {
  const typeMap: Record<number, NotificationType> = {
    1: "article_like",
    2: "comment_like",
    3: "comment_reply",
    4: "comment_reply",
  }

  return {
    id: notification.id,
    type: typeMap[notification.type] || "article_like",
    actor: {
      id: String(notification.sender_id),
      username: notification.sender_nickname,
      avatar: formatAvatarUrl(notification.sender_avatar),
      role: "user",
      location: "",
      joinedAt: new Date(notification.created_time * 1000).toISOString(),
    },
    read: notification.is_read,
    createdAt: new Date(notification.created_time * 1000).toISOString(),
    articleId: notification.article_id ? String(notification.article_id) : undefined,
    articleTitle: notification.title,
    actionText: notification.action_text,
  }
}
