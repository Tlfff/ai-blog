"use client"

import { useState } from "react"
import { ChevronLeft, ChevronRight, Loader2 } from "lucide-react"
import useSWR from "swr"
import { getArticles, searchArticles, type ArticleQuery } from "@/api/articles"
import type { Article, Role } from "@/types"
import { ArticleCard } from "./article-card"

const PAGE_SIZE = 10

export function ArticleList({ tag, keyword }: { tag?: string; keyword?: string }) {
  if (tag) return <TagSearchList tag={tag} />
  return <PublishedArticleList keyword={keyword} />
}

function PublishedArticleList({ keyword }: { keyword?: string }) {
  const [page, setPage] = useState(1)
  const query: ArticleQuery = { page, pageSize: PAGE_SIZE, keyword }
  const { data, isLoading } = useSWR(["articles", query], () => getArticles(query))

  if (isLoading && !data) return <ListLoading />
  if (!data || data.items.length === 0) return <ListEmpty />

  const startIndex = (page - 1) * PAGE_SIZE

  return (
    <div>
      {data.items.map((article, index) => (
        <ArticleCard key={article.id} article={article} index={startIndex + index} />
      ))}
      <DarkPagination
        page={data.page}
        total={data.total}
        pageSize={data.pageSize}
        onChange={setPage}
      />
    </div>
  )
}

function TagSearchList({ tag }: { tag: string }) {
  const [page, setPage] = useState(1)
  const { data, isLoading } = useSWR(
    ["article-tag-search", tag, page],
    () => searchArticles({ keyword: tag, page, pageSize: PAGE_SIZE }),
  )

  if (isLoading && !data) return <ListLoading />
  if (!data || data.items.length === 0) return <ListEmpty />

  const startIndex = (data.page - 1) * data.pageSize
  const articles: Article[] = data.items.map((item) => ({
    id: String(item.id),
    title: item.title,
    summary: item.summary,
    content: "",
    author: {
      id: "",
      username: "睦子米",
      avatar: "/avatars/admin.png",
      role: "user" as Role,
      location: "",
      joinedAt: "",
    },
    tags: item.tags.map((name) => ({ id: name, name, count: 0 })),
    status: "published",
    views: 0,
    likes: 0,
    commentsCount: 0,
    createdAt: "",
    updatedAt: "",
  }))

  return (
    <div>
      {articles.map((article, index) => (
        <ArticleCard key={article.id} article={article} index={startIndex + index} />
      ))}
      <DarkPagination
        page={data.page}
        total={data.total}
        pageSize={data.pageSize}
        onChange={setPage}
      />
    </div>
  )
}

function DarkPagination({
  page,
  total,
  pageSize,
  onChange,
}: {
  page: number
  total: number
  pageSize: number
  onChange: (page: number) => void
}) {
  const totalPages = Math.max(1, Math.ceil(total / pageSize))
  if (totalPages <= 1) return null

  return (
    <nav className="mt-10 flex items-center justify-center gap-4" aria-label="文章分页">
      <button
        type="button"
        onClick={() => onChange(page - 1)}
        disabled={page <= 1}
        className="grid size-9 place-items-center rounded-[4px] border border-[var(--home-divider)] text-[var(--home-muted)] transition-colors hover:border-[var(--home-accent)] hover:text-[var(--home-accent)] disabled:cursor-not-allowed disabled:opacity-30"
        aria-label="上一页"
      >
        <ChevronLeft className="size-4" />
      </button>
      <span className="text-xs tracking-[0.18em] text-[var(--home-faint)]">
        {String(page).padStart(2, "0")} / {String(totalPages).padStart(2, "0")}
      </span>
      <button
        type="button"
        onClick={() => onChange(page + 1)}
        disabled={page >= totalPages}
        className="grid size-9 place-items-center rounded-[4px] border border-[var(--home-divider)] text-[var(--home-muted)] transition-colors hover:border-[var(--home-accent)] hover:text-[var(--home-accent)] disabled:cursor-not-allowed disabled:opacity-30"
        aria-label="下一页"
      >
        <ChevronRight className="size-4" />
      </button>
    </nav>
  )
}

function ListLoading() {
  return (
    <div className="flex items-center justify-center gap-2 py-16 text-sm text-[var(--home-faint)]">
      <Loader2 className="size-4 animate-spin text-[var(--home-accent)]" />
      加载文章中...
    </div>
  )
}

function ListEmpty() {
  return <div className="py-16 text-center text-sm text-[var(--home-faint)]">没有找到相关文章</div>
}
