"use client"

import { useEffect, useState } from "react"
import Link from "next/link"
import { useRouter } from "next/navigation"
import { Trash2, Edit3, ArrowLeft, Send, ChevronLeft, ChevronRight } from "lucide-react"
import useSWR from "swr"
import { getAdminArticleList, deleteArticle, publishArticle } from "@/api/articles"
import { Container } from "@/components/layout/container"
import { SiteShell } from "@/components/layout/site-shell"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { useAuth } from "@/hooks/use-auth"
import { LoadingState } from "@/components/ui/spinner"
import { formatDate } from "@/lib/format"
import { cn } from "@/lib/utils"

export default function AdminDraftsPage() {
  const { isAdmin } = useAuth()
  const router = useRouter()
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

  const { data: drafts, isLoading, mutate } = useSWR(
    ["admin-drafts", page],
    () => getAdminArticleList(2, page),
  )

  async function handleDelete(id: string) {
    if (!confirm("确定要删除这个草稿吗？")) return
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

  const totalPages = drafts ? Math.ceil(drafts.total / drafts.pageSize) : 0

  return (
    <SiteShell>
      <Container className="py-8 md:py-12">
        <div className="mb-8 flex items-end justify-between gap-4 border-b border-border pb-6">
          <div className="flex items-center gap-3">
            <Link href="/admin" className="label-meta inline-flex items-center gap-2 text-ink transition-colors hover:text-sakura-deep">
              <ArrowLeft className="size-4" />
              back
            </Link>
            <div>
              <p className="label-meta text-sakura-deep">control room / drafts</p>
              <h1 className="title-display mt-2 text-4xl text-ink">草稿管理</h1>
            </div>
          </div>
          <Link href="/editor">
            <Button>新建文章</Button>
          </Link>
        </div>

        {isLoading ? (
          <LoadingState />
        ) : (
          <Card className="rounded-sm">
            <CardContent>
              {drafts?.items.length === 0 ? (
                <div className="py-12 text-center text-muted-foreground">暂无草稿</div>
              ) : (
                <>
                  <div className="flex flex-col gap-3">
                    {drafts?.items.map((draft) => (
                      <div
                        key={draft.id}
                        className="flex items-center justify-between gap-4 rounded-sm border border-border p-4 transition-colors hover:border-sakura-deep hover:bg-sakura-wash"
                      >
                        <div className="flex-1">
                          <h3 className="font-medium">{draft.title}</h3>
                          <p className="mt-1 text-sm text-muted-foreground">
                            作者：{draft.author.username} · 更新于 {formatDate(draft.updatedAt)}
                          </p>
                        </div>
                        <div className="flex flex-wrap items-center justify-end gap-2">
                          <Link href={`/editor?id=${draft.id}`}>
                            <Button variant="outline" size="sm">
                              <Edit3 className="size-3.5" />
                              编辑
                            </Button>
                          </Link>
                          <Button
                            variant="default"
                            size="sm"
                            onClick={() => handlePublish(draft.id)}
                            disabled={publishing === draft.id}
                          >
                            <Send className={cn("size-3.5", publishing === draft.id && "animate-pulse")} />
                            {publishing === draft.id ? "发布中..." : "发布"}
                          </Button>
                          <Button
                            variant="destructive"
                            size="sm"
                            onClick={() => handleDelete(draft.id)}
                            disabled={deleting === draft.id}
                          >
                            <Trash2 className={cn("size-3.5", deleting === draft.id && "animate-pulse")} />
                            删除
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
