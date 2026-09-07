"use client"

import { useState } from "react"
import { RefreshCw, Trash2 } from "lucide-react"
import useSWR from "swr"
import { getTrashList, hardDeleteArticle, recoverArticle } from "@/api/articles"
import { AdminShell } from "@/components/admin/admin-shell"
import { AdminEmptyState, AdminPageHeader, AdminPagination, AdminPanel, AdminStatusBadge } from "@/components/admin/admin-ui"
import { Button } from "@/components/ui/button"
import { LoadingState } from "@/components/ui/spinner"
import { useAuth } from "@/hooks/use-auth"
import { formatDate, formatNumber } from "@/lib/format"
import { cn } from "@/lib/utils"

export default function AdminTrashPage() {
  const { isAdmin } = useAuth()
  const [page, setPage] = useState(1)
  const [deleting, setDeleting] = useState<string | null>(null)
  const [restoring, setRestoring] = useState<string | null>(null)
  const { data: trashedArticles, isLoading, mutate } = useSWR(isAdmin ? ["admin-trash", page] : null, () => getTrashList(page))

  async function handleDelete(id: string) {
    if (!confirm("永久删除后无法恢复，确定继续吗？")) return
    setDeleting(id)
    try { await hardDeleteArticle(id); await mutate() } finally { setDeleting(null) }
  }

  async function handleRestore(id: string) {
    if (!confirm("确定要恢复这篇文章吗？文章将变为草稿状态。")) return
    setRestoring(id)
    try { await recoverArticle(id); await mutate() } finally { setRestoring(null) }
  }

  const totalPages = trashedArticles ? Math.ceil(trashedArticles.total / trashedArticles.pageSize) : 0

  return (
    <AdminShell>
      <AdminPageHeader eyebrow="control room / recycle bin" title="回收站" description="恢复误删文章，或进行不可撤销的永久删除。" />
      {isLoading ? <LoadingState /> : (
        <AdminPanel className="overflow-hidden">
          {trashedArticles?.items.length ? (
            <>
              <div className="border-b border-[var(--admin-border)] bg-[var(--admin-coral-soft)] px-5 py-4 text-sm text-[var(--admin-coral)] sm:px-6"><Trash2 className="mr-2 inline size-4" />回收站中的文章不会出现在公开站点。</div>
              <div className="divide-y divide-[var(--admin-border)]">
                {trashedArticles.items.map((article) => (
                  <article key={article.id} className="flex flex-col gap-4 p-5 transition-colors hover:bg-[var(--admin-coral-soft)]/35 sm:flex-row sm:items-center sm:px-6">
                    <div className="min-w-0 flex-1"><div className="flex flex-wrap items-center gap-2"><h2 className="font-semibold text-[var(--admin-muted)] line-through">{article.title}</h2><AdminStatusBadge status="deleted" /></div><p className="mt-2 text-xs text-[var(--admin-faint)]">删除于 {formatDate(article.updatedAt)} · {formatNumber(article.views)} 阅读 / {formatNumber(article.likes)} 点赞 / {article.commentsCount} 评论</p></div>
                    <div className="flex flex-wrap items-center gap-2 sm:justify-end"><Button variant="outline" size="sm" onClick={() => handleRestore(article.id)} disabled={restoring === article.id} className="rounded-full border-[var(--admin-border-strong)] bg-transparent"><RefreshCw className={cn("size-3.5", restoring === article.id && "animate-spin")} />{restoring === article.id ? "恢复中" : "恢复为草稿"}</Button><Button size="sm" onClick={() => handleDelete(article.id)} disabled={deleting === article.id} className="rounded-full bg-[var(--admin-coral)] text-white hover:opacity-85"><Trash2 className={cn("size-3.5", deleting === article.id && "animate-pulse")} />永久删除</Button></div>
                  </article>
                ))}
              </div>
              <AdminPagination page={page} totalPages={totalPages} onPageChange={setPage} />
            </>
          ) : <AdminEmptyState title="回收站是空的" description="没有等待恢复或永久删除的文章。" />}
        </AdminPanel>
      )}
    </AdminShell>
  )
}
