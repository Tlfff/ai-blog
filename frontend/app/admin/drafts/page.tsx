"use client"

import { useState } from "react"
import Link from "next/link"
import { Edit3, FileText, PenLine, Send, Trash2 } from "lucide-react"
import useSWR from "swr"
import { deleteArticle, getAdminArticleList, publishArticle } from "@/api/articles"
import { AdminShell } from "@/components/admin/admin-shell"
import { AdminEmptyState, AdminPageHeader, AdminPagination, AdminPanel, AdminStatusBadge } from "@/components/admin/admin-ui"
import { Button } from "@/components/ui/button"
import { LoadingState } from "@/components/ui/spinner"
import { useAuth } from "@/hooks/use-auth"
import { formatDate } from "@/lib/format"
import { cn } from "@/lib/utils"

export default function AdminDraftsPage() {
  const { isAdmin } = useAuth()
  const [deleting, setDeleting] = useState<string | null>(null)
  const [publishing, setPublishing] = useState<string | null>(null)
  const [page, setPage] = useState(1)
  const { data: drafts, isLoading, mutate } = useSWR(isAdmin ? ["admin-drafts", page] : null, () => getAdminArticleList(2, page))

  async function handleDelete(id: string) {
    if (!confirm("确定要删除这个草稿吗？")) return
    setDeleting(id)
    try { await deleteArticle(id); await mutate() } finally { setDeleting(null) }
  }

  async function handlePublish(id: string) {
    setPublishing(id)
    try { await publishArticle(id); await mutate() } finally { setPublishing(null) }
  }

  const totalPages = drafts ? Math.ceil(drafts.total / drafts.pageSize) : 0

  return (
    <AdminShell>
      <AdminPageHeader eyebrow="control room / drafts" title="草稿箱" description="继续完成尚未发布的内容。" actions={<Link href="/editor"><Button className="rounded-full bg-[var(--admin-sky)] text-white hover:bg-[var(--admin-sky-deep)]"><PenLine className="size-4" />新建文章</Button></Link>} />
      {isLoading ? <LoadingState /> : (
        <AdminPanel className="overflow-hidden">
          {drafts?.items.length ? (
            <>
              <div className="border-b border-[var(--admin-border)] bg-[var(--admin-yellow-soft)] px-5 py-4 text-sm text-[var(--admin-yellow-deep)] sm:px-6"><FileText className="mr-2 inline size-4" />共有 {drafts.total} 篇草稿，发布前可以继续编辑和检查。</div>
              <div className="divide-y divide-[var(--admin-border)]">
                {drafts.items.map((draft) => (
                  <article key={draft.id} className="flex flex-col gap-4 p-5 transition-colors hover:bg-[var(--admin-sky-soft)]/35 sm:flex-row sm:items-center sm:px-6">
                    <div className="min-w-0 flex-1"><div className="flex flex-wrap items-center gap-2"><h2 className="font-semibold text-[var(--admin-ink)]">{draft.title}</h2><AdminStatusBadge status="draft" /></div><p className="mt-2 text-xs text-[var(--admin-faint)]">更新于 {formatDate(draft.updatedAt)} · {draft.tags.map((tag) => tag.name).join(" · ") || "暂无标签"}</p></div>
                    <div className="flex flex-wrap items-center gap-2 sm:justify-end"><Link href={`/editor?id=${draft.id}`}><Button variant="outline" size="sm" className="rounded-full border-[var(--admin-border-strong)] bg-transparent"><Edit3 className="size-3.5" />继续写</Button></Link><Button size="sm" onClick={() => handlePublish(draft.id)} disabled={publishing === draft.id} className="rounded-full bg-[var(--admin-teal-deep)] text-white"><Send className={cn("size-3.5", publishing === draft.id && "animate-pulse")} />{publishing === draft.id ? "发布中" : "发布"}</Button><Button variant="ghost" size="sm" onClick={() => handleDelete(draft.id)} disabled={deleting === draft.id} className="rounded-full text-[var(--admin-coral)] hover:bg-[var(--admin-coral-soft)] hover:text-[var(--admin-coral)]"><Trash2 className={cn("size-3.5", deleting === draft.id && "animate-pulse")} />删除</Button></div>
                  </article>
                ))}
              </div>
              <AdminPagination page={page} totalPages={totalPages} onPageChange={setPage} />
            </>
          ) : <AdminEmptyState title="草稿箱是空的" description="所有灵感都已经整理发布，或者还没开始书写。" />}
        </AdminPanel>
      )}
    </AdminShell>
  )
}
