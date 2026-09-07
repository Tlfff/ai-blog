"use client"

import { useState } from "react"
import Link from "next/link"
import { MessageSquare, Trash2 } from "lucide-react"
import useSWR from "swr"
import { adminDeleteComment, getAdminCommentCollection } from "@/api/comments"
import { AdminShell } from "@/components/admin/admin-shell"
import { AdminEmptyState, AdminPageHeader, AdminPagination, AdminPanel, AdminPanelHeading } from "@/components/admin/admin-ui"
import { Button } from "@/components/ui/button"
import { LoadingState } from "@/components/ui/spinner"
import { useAuth } from "@/hooks/use-auth"
import { formatDate } from "@/lib/format"
import { cn } from "@/lib/utils"

const PAGE_SIZE = 20

export default function AdminCommentsPage() {
  const { isAdmin } = useAuth()
  const [deleting, setDeleting] = useState<string | null>(null)
  const [page, setPage] = useState(1)
  const { data: collection, isLoading, mutate } = useSWR(isAdmin ? "admin-comment-collection" : null, getAdminCommentCollection)
  const allComments = collection?.comments ?? []
  const totalPages = Math.max(1, Math.ceil(allComments.length / PAGE_SIZE))
  const pageComments = allComments.slice((page - 1) * PAGE_SIZE, page * PAGE_SIZE)
  const articleMap = new Map(collection?.articles.map((article) => [article.id, article]) ?? [])

  async function handleDelete(id: string) {
    if (!confirm("确定要删除这条评论吗？")) return
    setDeleting(id)
    try { await adminDeleteComment(id); await mutate() } finally { setDeleting(null) }
  }

  return (
    <AdminShell>
      <AdminPageHeader eyebrow="control room / community" title="评论管理" description="查看并处理文章下的用户互动。" />
      {isLoading ? <LoadingState /> : (
        <AdminPanel className="overflow-hidden">
          <AdminPanelHeading title="全部评论" description={`共 ${allComments.length} 条站内互动`} />
          {pageComments.length ? (
            <>
              <div className="divide-y divide-[var(--admin-border)]">
                {pageComments.map((comment) => {
                  const article = articleMap.get(comment.articleId)
                  return (
                    <article key={comment.id} className="flex flex-col gap-4 p-5 transition-colors hover:bg-[var(--admin-sky-soft)]/35 sm:flex-row sm:items-start sm:px-6">
                      <span className="grid size-10 shrink-0 place-items-center rounded-full bg-[var(--admin-teal-soft)] text-sm font-bold text-[var(--admin-teal-deep)]">{comment.author.username.slice(0, 1)}</span>
                      <div className="min-w-0 flex-1">
                        <div className="flex flex-wrap items-center gap-2"><strong className="text-sm text-[var(--admin-ink)]">{comment.author.username}</strong>{comment.parentId ? <span className="rounded-full bg-[var(--admin-lavender-soft)] px-2 py-0.5 text-[0.65rem] text-[var(--admin-lavender)]">回复</span> : null}<span className="text-xs text-[var(--admin-faint)]">{formatDate(comment.createdAt)}</span></div>
                        <p className="mt-2 text-sm leading-6 text-[var(--admin-muted)]"><MessageSquare className="mr-1.5 inline size-3.5" />{comment.content}</p>
                        <p className="mt-2 text-xs text-[var(--admin-faint)]">所属文章：{article ? <Link href={`/articles/${article.id}`} className="text-[var(--admin-sky-deep)] hover:underline">{article.title}</Link> : "未知文章"}</p>
                      </div>
                      <Button variant="ghost" size="sm" onClick={() => handleDelete(comment.id)} disabled={deleting === comment.id} className="self-end rounded-full text-[var(--admin-coral)] hover:bg-[var(--admin-coral-soft)] hover:text-[var(--admin-coral)] sm:self-start"><Trash2 className={cn("size-3.5", deleting === comment.id && "animate-pulse")} />删除</Button>
                    </article>
                  )
                })}
              </div>
              <AdminPagination page={page} totalPages={totalPages} onPageChange={setPage} />
            </>
          ) : <AdminEmptyState title="暂无评论" description="文章还没有收到评论。" />}
        </AdminPanel>
      )}
    </AdminShell>
  )
}
