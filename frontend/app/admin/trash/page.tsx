"use client"

import { useEffect, useState } from "react"
import Link from "next/link"
import { useRouter } from "next/navigation"
import { Trash2, RefreshCw, ArrowLeft, ChevronLeft, ChevronRight } from "lucide-react"
import useSWR from "swr"
import { getTrashList, recoverArticle, hardDeleteArticle } from "@/api/articles"
import { Container } from "@/components/layout/container"
import { SiteShell } from "@/components/layout/site-shell"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { useAuth } from "@/hooks/use-auth"
import { LoadingState } from "@/components/ui/spinner"
import { formatDate, formatNumber } from "@/lib/format"
import { cn } from "@/lib/utils"

export default function AdminTrashPage() {
  const { isAdmin } = useAuth()
  const router = useRouter()
  const [deleting, setDeleting] = useState<string | null>(null)
  const [restoring, setRestoring] = useState<string | null>(null)
  const [page, setPage] = useState(1)

  useEffect(() => {
    if (!isAdmin) {
      router.push("/")
    }
  }, [isAdmin, router])

  if (!isAdmin) {
    return null
  }

  const { data: trashedArticles, isLoading, mutate } = useSWR(
    ["admin-trash", page],
    () => getTrashList(page),
  )

  async function handleDelete(id: string) {
    if (!confirm("确定要永久删除这篇文章吗？此操作不可恢复！")) return
    setDeleting(id)
    try {
      await hardDeleteArticle(id)
      mutate()
    } finally {
      setDeleting(null)
    }
  }

  async function handleRestore(id: string) {
    if (!confirm("确定要恢复这篇文章吗？文章将变为草稿状态。")) return
    setRestoring(id)
    try {
      await recoverArticle(id)
      mutate()
    } finally {
      setRestoring(null)
    }
  }

  const totalPages = trashedArticles ? Math.ceil(trashedArticles.total / trashedArticles.pageSize) : 0

  return (
    <SiteShell>
      <Container className="py-8 md:py-12">
        <div className="mb-8 flex items-end justify-between border-b border-border pb-6">
          <div className="flex items-center gap-3">
            <Link href="/admin" className="label-meta inline-flex items-center gap-2 text-ink transition-colors hover:text-sakura-deep">
              <ArrowLeft className="size-4" />
              back
            </Link>
            <div>
              <p className="label-meta text-sakura-deep">control room / recycle bin</p>
              <h1 className="title-display mt-2 text-4xl text-ink">垃圾箱</h1>
            </div>
          </div>
        </div>

        {isLoading ? (
          <LoadingState />
        ) : (
          <Card className="rounded-sm">
            <CardContent>
              {trashedArticles?.items.length === 0 ? (
                <div className="py-12 text-center text-muted-foreground">
                  <Trash2 className="mx-auto mb-4 size-12 opacity-50" />
                  <p>垃圾箱为空</p>
                </div>
              ) : (
                <>
                  <div className="flex flex-col gap-3">
                    {trashedArticles?.items.map((article) => (
                      <div
                        key={article.id}
                        className="flex items-center justify-between gap-4 rounded-sm border border-destructive/30 bg-destructive/5 p-4 transition-colors hover:bg-destructive/10"
                      >
                        <div className="flex-1">
                          <h3 className="font-medium line-through text-muted-foreground">{article.title}</h3>
                          <div className="mt-1 flex flex-wrap gap-2">
                            {article.tags.map((tag) => (
                              <span key={tag.id} className="rounded-full bg-muted/50 px-2 py-0.5 text-xs text-muted-foreground">
                                {tag.name}
                              </span>
                            ))}
                          </div>
                          <p className="mt-1 text-sm text-muted-foreground/70">
                            {formatNumber(article.views)}阅 / {formatNumber(article.likes)}赞 / {article.commentsCount}评 · 删除于 {formatDate(article.updatedAt)}
                          </p>
                        </div>
                        <div className="flex flex-wrap items-center justify-end gap-2">
                          <Button
                            variant="outline"
                            size="sm"
                            onClick={() => handleRestore(article.id)}
                            disabled={restoring === article.id}
                          >
                            <RefreshCw className={cn("size-3.5", restoring === article.id && "animate-spin")} />
                            {restoring === article.id ? "恢复中..." : "恢复"}
                          </Button>
                          <Button
                            variant="destructive"
                            size="sm"
                            onClick={() => handleDelete(article.id)}
                            disabled={deleting === article.id}
                          >
                            <Trash2 className={cn("size-3.5", deleting === article.id && "animate-pulse")} />
                            永久删除
                          </Button>
                        </div>
                      </div>
                    ))}
                  </div>

                  {totalPages > 1 && (
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
                </>
              )}
            </CardContent>
          </Card>
        )}
      </Container>
    </SiteShell>
  )
}
