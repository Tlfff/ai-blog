"use client"

import { useEffect, useState } from "react"
import Link from "next/link"
import { useRouter, useSearchParams } from "next/navigation"
import { ArrowLeft, ChevronLeft, ChevronRight, MessageSquare, Search, Trash2 } from "lucide-react"
import useSWR from "swr"
import { adminDeleteComment, getAdminCommentCollection } from "@/api/comments"
import { Container } from "@/components/layout/container"
import { SiteShell } from "@/components/layout/site-shell"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { useAuth } from "@/hooks/use-auth"
import { LoadingState } from "@/components/ui/spinner"
import { formatDate } from "@/lib/format"
import { cn } from "@/lib/utils"

const PAGE_SIZE = 20

export default function AdminCommentsPage() {
  const { isAdmin } = useAuth()
  const router = useRouter()
  const searchParams = useSearchParams()
  const [keyword, setKeyword] = useState(searchParams.get("q") || "")
  const [deleting, setDeleting] = useState<string | null>(null)
  const [page, setPage] = useState(1)

  useEffect(() => {
    if (!isAdmin) {
      router.push("/")
    }
  }, [isAdmin, router])

  if (!isAdmin) {
    return null
  }

  const { data: collection, isLoading, mutate } = useSWR(
    "admin-comment-collection",
    getAdminCommentCollection,
  )
  const allComments = collection?.comments ?? []

  const filteredComments = keyword
    ? allComments.filter((c) => c.content.toLowerCase().includes(keyword.toLowerCase()))
    : allComments
  const totalPages = Math.max(1, Math.ceil(filteredComments.length / PAGE_SIZE))
  const pageComments = filteredComments.slice((page - 1) * PAGE_SIZE, page * PAGE_SIZE)
  const articleMap = new Map(collection?.articles.map((article) => [article.id, article]) ?? [])

  async function handleDelete(id: string) {
    if (!confirm("确定要删除这条评论吗？")) return
    setDeleting(id)
    try {
      await adminDeleteComment(id)
      await mutate()
    } finally {
      setDeleting(null)
    }
  }

  function handleSearch() {
    setPage(1)
    if (keyword) {
      router.push(`/admin/comments?q=${encodeURIComponent(keyword)}`)
    } else {
      router.push("/admin/comments")
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
              <p className="label-meta text-sakura-deep">control room / community</p>
              <h1 className="title-display mt-2 text-4xl text-ink">评论管理</h1>
            </div>
          </div>

          <div className="flex w-full flex-wrap items-center gap-2 md:w-auto">
            <div className="relative">
              <Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
              <input
                type="text"
                value={keyword}
                onChange={(e) => setKeyword(e.target.value)}
                onKeyDown={(e) => e.key === "Enter" && handleSearch()}
                placeholder="搜索评论内容..."
                className="h-9 w-full rounded-sm border border-border bg-paper pl-9 pr-3 text-sm outline-none focus:border-sakura-deep focus:ring-2 focus:ring-sakura/30 sm:w-64"
              />
            </div>
            <Button onClick={handleSearch}>搜索</Button>
          </div>
        </div>

        {!isLoading && collection ? (
          <Card className="rounded-sm">
            <CardContent>
              <div className="overflow-x-auto">
              <table className="w-full min-w-[760px] text-sm">
                <thead>
                  <tr className="border-b-2 border-border">
                    <th className="px-4 py-3 text-left font-medium">内容</th>
                    <th className="px-4 py-3 text-left font-medium">评论者昵称</th>
                    <th className="px-4 py-3 text-left font-medium">所属文章</th>
                    <th className="px-4 py-3 text-left font-medium">创建时间</th>
                    <th className="px-4 py-3 text-right font-medium">操作</th>
                  </tr>
                </thead>
                <tbody>
                  {pageComments.map((comment) => {
                    const article = articleMap.get(comment.articleId)
                    return (
                      <tr key={comment.id} className="border-b border-border/50 transition-colors hover:bg-sakura-wash/50">
                        <td className="px-4 py-3 max-w-md">
                          <div className="flex items-start gap-2">
                            <MessageSquare className="mt-0.5 size-4 text-muted-foreground shrink-0" />
                            <p className="line-clamp-2">{comment.content}</p>
                          </div>
                          {comment.parentId && (
                            <span className="ml-6 text-xs text-muted-foreground">（回复）</span>
                          )}
                        </td>
                        <td className="px-4 py-3">{comment.author.username}</td>
                        <td className="px-4 py-3">
                          {article ? (
                            <Link href={`/articles/${article.id}`} className="text-primary hover:underline">
                              {article.title}
                            </Link>
                          ) : (
                            <span className="text-muted-foreground">未知文章</span>
                          )}
                        </td>
                        <td className="px-4 py-3 text-muted-foreground">{formatDate(comment.createdAt)}</td>
                        <td className="px-4 py-3">
                          <div className="flex items-center justify-end">
                            <Button
                              variant="destructive"
                              size="icon-xs"
                              onClick={() => handleDelete(comment.id)}
                              disabled={deleting === comment.id}
                              aria-label="删除"
                            >
                              <Trash2 className={cn("size-3.5", deleting === comment.id && "animate-pulse")} />
                            </Button>
                          </div>
                        </td>
                      </tr>
                    )
                  })}
                </tbody>
              </table>
              </div>

              {pageComments.length === 0 && (
                <div className="py-12 text-center text-muted-foreground">暂无评论</div>
              )}

              {totalPages > 1 && (
                <div className="mt-6 flex items-center justify-center gap-2">
                  <Button
                    variant="outline"
                    size="sm"
                    disabled={page <= 1}
                    onClick={() => setPage((current) => current - 1)}
                  >
                    <ChevronLeft className="size-4" />
                  </Button>
                  <span className="text-sm text-muted-foreground">
                    第 {page} / {totalPages} 页
                  </span>
                  <Button
                    variant="outline"
                    size="sm"
                    disabled={page >= totalPages}
                    onClick={() => setPage((current) => current + 1)}
                  >
                    <ChevronRight className="size-4" />
                  </Button>
                </div>
              )}
            </CardContent>
          </Card>
        ) : (
          <LoadingState />
        )}
      </Container>
    </SiteShell>
  )
}
