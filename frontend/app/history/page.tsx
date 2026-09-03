"use client"

import { useState } from "react"
import Link from "next/link"
import useSWR from "swr"
import { Clock, ArrowLeft } from "lucide-react"
import { getHistory } from "@/api/users"
import { Container } from "@/components/layout/container"
import { SiteShell } from "@/components/layout/site-shell"
import { Card, CardContent } from "@/components/ui/card"
import { LoadingState, EmptyState } from "@/components/ui/spinner"
import { formatDate } from "@/lib/format"
import { useAuth } from "@/hooks/use-auth"

export default function HistoryPage() {
  const { isLoggedIn } = useAuth()
  const [page, setPage] = useState(1)

  if (!isLoggedIn) {
    return (
      <SiteShell>
        <Container className="py-12 text-center">
          <Clock className="mx-auto mb-4 size-12 text-muted-foreground" />
          <p className="text-muted-foreground">请先登录查看浏览历史</p>
        </Container>
      </SiteShell>
    )
  }

  const { data: history, isLoading } = useSWR(["history", page], () => getHistory(page))

  if (isLoading && !history) return <LoadingState />
  if (!history || history.items.length === 0) {
    return (
      <SiteShell>
        <Container className="py-12 text-center">
          <Clock className="mx-auto mb-4 size-12 text-muted-foreground" />
          <p className="text-muted-foreground">暂无浏览记录</p>
        </Container>
      </SiteShell>
    )
  }

  return (
    <SiteShell>
      <Container className="py-6">
        <div className="flex items-center gap-3">
          <Link href="/" className="flex items-center gap-1 rounded-md px-2 py-1 hover:bg-muted">
            <ArrowLeft className="size-4" />
            返回
          </Link>
          <h1 className="text-xl font-semibold">浏览历史</h1>
        </div>

        <div className="mt-6 space-y-3">
          {history.items.map((item) => (
            <Card key={item.articleId} className="transition-all duration-200 hover:shadow-lg">
              <CardContent className="flex items-center gap-4 p-4">
                <div className="flex h-12 w-12 shrink-0 items-center justify-center rounded-lg bg-primary/10">
                  <Clock className="size-5 text-primary" />
                </div>
                <div className="flex-1 min-w-0">
                  <Link href={`/articles/${item.articleId}`} className="font-medium hover:text-primary truncate">
                    {item.title}
                  </Link>
                  <p className="mt-1 text-sm text-muted-foreground">
                    浏览于 {formatDate(item.viewedAt)}
                  </p>
                </div>
              </CardContent>
            </Card>
          ))}
        </div>

        {history.total > history.pageSize && (
          <div className="mt-6 flex justify-center">
            <div className="flex items-center gap-2 rounded-lg border border-border p-1">
              {page > 1 && (
                <button
                  onClick={() => setPage(page - 1)}
                  className="rounded-md px-3 py-1.5 text-sm hover:bg-muted"
                >
                  上一页
                </button>
              )}
              <span className="px-3 py-1.5 text-sm text-muted-foreground">
                第 {page} 页 / 共 {Math.ceil(history.total / history.pageSize)} 页
              </span>
              {page < Math.ceil(history.total / history.pageSize) && (
                <button
                  onClick={() => setPage(page + 1)}
                  className="rounded-md px-3 py-1.5 text-sm hover:bg-muted"
                >
                  下一页
                </button>
              )}
            </div>
          </div>
        )}
      </Container>
    </SiteShell>
  )
}