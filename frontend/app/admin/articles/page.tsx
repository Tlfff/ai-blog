"use client"

import { Suspense, useState } from "react"
import { useEffect } from "react"
import Link from "next/link"
import { useRouter, useSearchParams } from "next/navigation"
import { Trash2, Edit3, Eye, ArrowLeft, Search, ChevronLeft, ChevronRight, Send } from "lucide-react"
import useSWR from "swr"
import { getAdminArticleList, deleteArticle, publishArticle } from "@/api/articles"
import { Container } from "@/components/layout/container"
import { SiteShell } from "@/components/layout/site-shell"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { useAuth } from "@/hooks/use-auth"
import { LoadingState } from "@/components/ui/spinner"
import { formatDate, formatNumber } from "@/lib/format"
import { cn } from "@/lib/utils"

const STATUS_OPTIONS = [
  { value: -1, label: "全部" },
  { value: 3, label: "已发表" },
  { value: 2, label: "草稿" },
]

function AdminArticlesContent() {
  const { isAdmin } = useAuth()
  const router = useRouter()
  const searchParams = useSearchParams()
  const [keyword, setKeyword] = useState(searchParams.get("q") || "")
  const activeKeyword = searchParams.get("q")?.trim() ?? ""
  const [status, setStatus] = useState<number>(parseInt(searchParams.get("status") || "-1"))
  const [deleting, setDeleting] = useState<string | null>(null)
  const [publishing, setPublishing] = useState<string | null>(null)
  const [page, setPage] = useState(1)

  useEffect(() => {
    if (!isAdmin) {
      router.push("/")
    }
  }, [isAdmin, router])

  if (!isAdmin) {
    return null
  }

  const { data: articles, isLoading, mutate } = useSWR(
    ["admin-articles", activeKeyword, status, page],
    () => getAdminArticleList(status, page, 10, activeKeyword),
  )

  async function handleDelete(id: string) {
    if (!confirm("确定要删除这篇文章吗？")) return
    setDeleting(id)
    try {
      await deleteArticle(id)
      mutate()
    } finally {
      setDeleting(null)
    }
  }

  async function handlePublish(id: string) {
    setPublishing(id)
    try {
      await publishArticle(id)
      mutate()
    } finally {
      setPublishing(null)
    }
  }

  function handleSearch() {
    setPage(1)
    const params = new URLSearchParams()
    if (keyword) params.set("q", keyword)
    if (status !== -1) params.set("status", String(status))
    router.push(`/admin/articles?${params.toString()}`)
  }

  function handleStatusChange(e: React.ChangeEvent<HTMLSelectElement>) {
    setStatus(parseInt(e.target.value))
    setPage(1)
    const params = new URLSearchParams()
    if (keyword) params.set("q", keyword)
    params.set("status", e.target.value)
    router.push(`/admin/articles?${params.toString()}`)
  }

  const totalPages = articles ? Math.ceil(articles.total / articles.pageSize) : 0

  function getStatusLabel(status: string) {
    switch (status) {
      case "published":
        return "已发表"
      case "draft":
        return "草稿"
      default:
        return "已删除"
    }
  }

  return (
    <SiteShell>
      <Container className="py-8 md:py-12">
        <div className="mb-8 flex flex-col items-start justify-between gap-5 border-b border-border pb-6 md:flex-row md:items-end">
          <div className="flex items-center gap-3">
            <Link href="/admin" className="label-meta inline-flex items-center gap-2 text-ink transition-colors hover:text-sakura-deep">
              <ArrowLeft className="size-4" />
              back
            </Link>
            <div>
              <p className="label-meta text-sakura-deep">control room / archive</p>
              <h1 className="title-display mt-2 text-4xl text-ink">文章管理</h1>
            </div>
          </div>

          <div className="flex w-full flex-wrap items-center gap-2 md:w-auto">
            <select
              value={status}
              onChange={handleStatusChange}
              className="h-9 rounded-sm border border-border bg-paper px-3 text-sm outline-none focus:border-sakura-deep focus:ring-2 focus:ring-sakura/30"
            >
              {STATUS_OPTIONS.map((option) => (
                <option key={option.value} value={option.value}>
                  {option.label}
                </option>
              ))}
            </select>

            <div className="relative">
              <Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
              <input
                type="text"
                value={keyword}
                onChange={(e) => setKeyword(e.target.value)}
                onKeyDown={(e) => e.key === "Enter" && handleSearch()}
                placeholder="搜索文章..."
                className="h-9 w-full rounded-sm border border-border bg-paper pl-9 pr-3 text-sm outline-none focus:border-sakura-deep focus:ring-2 focus:ring-sakura/30 sm:w-64"
              />
            </div>
            <Button onClick={handleSearch}>搜索</Button>
          </div>
        </div>

        {isLoading ? (
          <LoadingState />
        ) : (
          <Card className="rounded-sm">
            <CardContent>
              <div className="overflow-x-auto">
              <table className="w-full min-w-[760px] text-sm">
                <thead>
                  <tr className="border-b-2 border-border">
                    <th className="px-4 py-3 text-left font-medium">标题</th>
                    <th className="px-4 py-3 text-left font-medium">标签</th>
                    <th className="px-4 py-3 text-left font-medium">状态</th>
                    <th className="px-4 py-3 text-left font-medium">阅读/点赞/评论</th>
                    <th className="px-4 py-3 text-left font-medium">更新时间</th>
                    <th className="px-4 py-3 text-right font-medium">操作</th>
                  </tr>
                </thead>
                <tbody>
                  {articles?.items.map((article) => (
                    <tr key={article.id} className="border-b border-border/50 transition-colors hover:bg-sakura-wash/50">
                      <td className="px-4 py-3">
                        <Link href={`/articles/${article.id}`} className="font-medium hover:text-primary">
                          {article.title}
                        </Link>
                      </td>
                      <td className="px-4 py-3">
                        {article.tags.map((tag) => (
                          <span key={tag.id} className="mr-1 rounded-sm border border-border bg-transparent px-2 py-0.5 text-xs text-ink-soft">
                            {tag.name}
                          </span>
                        ))}
                      </td>
                      <td className="px-4 py-3">
                        <span
                          className={cn(
                            "rounded-sm px-2 py-0.5 text-xs",
                            article.status === "published"
                              ? "bg-green-100 text-green-700"
                              : article.status === "draft"
                              ? "bg-yellow-100 text-yellow-700"
                              : "bg-red-100 text-red-700",
                          )}
                        >
                          {getStatusLabel(article.status)}
                        </span>
                      </td>
                      <td className="px-4 py-3 text-muted-foreground">
                        {formatNumber(article.views)} / {formatNumber(article.likes)} / {article.commentsCount}
                      </td>
                      <td className="px-4 py-3 text-muted-foreground">{formatDate(article.updatedAt)}</td>
                      <td className="px-4 py-3">
                        <div className="flex items-center justify-end gap-2">
                          {article.status !== "deleted" && (
                            <Link href={`/articles/${article.id}`}>
                              <Button variant="ghost" size="icon-xs" aria-label="预览">
                                <Eye className="size-3.5" />
                              </Button>
                            </Link>
                          )}
                          <Link href={`/editor?id=${article.id}`}>
                            <Button variant="ghost" size="icon-xs" aria-label="编辑">
                              <Edit3 className="size-3.5" />
                            </Button>
                          </Link>
                          {article.status === "draft" && (
                            <Button
                              variant="default"
                              size="icon-xs"
                              onClick={() => handlePublish(article.id)}
                              disabled={publishing === article.id}
                              aria-label="发布"
                            >
                              <Send className={cn("size-3.5", publishing === article.id && "animate-pulse")} />
                            </Button>
                          )}
                            <Button
                              variant="destructive"
                              size="icon-xs"
                              onClick={() => handleDelete(article.id)}
                              disabled={deleting === article.id}
                              aria-label="删除"
                            >
                              <Trash2 className={cn("size-3.5", deleting === article.id && "animate-pulse")} />
                            </Button>
                        </div>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
              </div>

              {articles?.items.length === 0 && (
                <div className="py-12 text-center text-muted-foreground">暂无文章</div>
              )}

              {articles && totalPages > 1 && (
                <div className="mt-6 flex items-center justify-center gap-2">
                  <Button
                    variant="outline"
                    size="sm"
                    disabled={page === 1}
                    onClick={() => setPage(page - 1)}
                  >
                    <ChevronLeft className="size-4" />
                  </Button>
                  <span className="text-sm text-muted-foreground">
                    第 {page} / {totalPages} 页
                  </span>
                  <Button
                    variant="outline"
                    size="sm"
                    disabled={page === totalPages}
                    onClick={() => setPage(page + 1)}
                  >
                    <ChevronRight className="size-4" />
                  </Button>
                </div>
              )}
            </CardContent>
          </Card>
        )}
      </Container>
    </SiteShell>
  )
}

export default function AdminArticlesPage() {
  return (
    <Suspense fallback={<LoadingState />}>
      <AdminArticlesContent />
    </Suspense>
  )
}
