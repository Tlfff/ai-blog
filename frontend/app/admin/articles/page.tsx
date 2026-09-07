"use client"

import { useState } from "react"
import Link from "next/link"
import { Edit3, Eye, PenLine, Send, Trash2 } from "lucide-react"
import useSWR from "swr"
import { deleteArticle, getAdminArticleList, publishArticle } from "@/api/articles"
import { AdminShell } from "@/components/admin/admin-shell"
import { AdminEmptyState, AdminPageHeader, AdminPagination, AdminPanel, AdminStatusBadge, adminInputClass } from "@/components/admin/admin-ui"
import { Button } from "@/components/ui/button"
import { LoadingState } from "@/components/ui/spinner"
import { useAuth } from "@/hooks/use-auth"
import { formatDate, formatNumber } from "@/lib/format"
import { cn } from "@/lib/utils"

const STATUS_OPTIONS = [{ value: -1, label: "全部状态" }, { value: 3, label: "已发表" }, { value: 2, label: "草稿" }]

export default function AdminArticlesPage() {
  const { isAdmin } = useAuth()
  const [status, setStatus] = useState(-1)
  const [deleting, setDeleting] = useState<string | null>(null)
  const [publishing, setPublishing] = useState<string | null>(null)
  const [page, setPage] = useState(1)
  const { data: articles, isLoading, mutate } = useSWR(
    isAdmin ? ["admin-articles", status, page] : null,
    () => getAdminArticleList(status, page, 10),
  )

  async function handleDelete(id: string) {
    if (!confirm("确定要删除这篇文章吗？")) return
    setDeleting(id)
    try { await deleteArticle(id); await mutate() } finally { setDeleting(null) }
  }

  async function handlePublish(id: string) {
    setPublishing(id)
    try { await publishArticle(id); await mutate() } finally { setPublishing(null) }
  }

  const totalPages = articles ? Math.ceil(articles.total / articles.pageSize) : 0

  return (
    <AdminShell>
      <AdminPageHeader eyebrow="control room / archive" title="文章管理" description="集中管理已发表文章与草稿。" actions={<Link href="/editor"><Button className="rounded-full bg-[var(--admin-sky)] text-white hover:bg-[var(--admin-sky-deep)]"><PenLine className="size-4" />新建文章</Button></Link>} />

      <AdminPanel className="mb-5 flex items-center gap-3 p-4">
        <select value={status} onChange={(event) => { setStatus(Number(event.target.value)); setPage(1) }} className={cn(adminInputClass, "min-w-36 pr-8")}>
          {STATUS_OPTIONS.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}
        </select>
        <span className="text-xs text-[var(--admin-faint)] sm:ml-auto">共 {articles?.total ?? "—"} 篇</span>
      </AdminPanel>

      {isLoading ? <LoadingState /> : (
        <AdminPanel className="overflow-hidden">
          {articles?.items.length ? (
            <>
              <div className="hidden overflow-x-auto md:block">
                <table className="w-full min-w-[1120px] text-sm">
                  <thead><tr className="border-b border-[var(--admin-border)] bg-[var(--admin-input)] text-left text-xs text-[var(--admin-faint)]"><th className="px-6 py-4 font-medium">标题</th><th className="px-4 py-4 font-medium">标签</th><th className="px-4 py-4 font-medium">状态</th><th className="px-4 py-4 font-medium">阅读 / 点赞 / 评论</th><th className="px-4 py-4 font-medium">创建时间</th><th className="px-4 py-4 font-medium">更新时间</th><th className="px-6 py-4 text-right font-medium">操作</th></tr></thead>
                  <tbody className="divide-y divide-[var(--admin-border)]">
                    {articles.items.map((article) => <ArticleTableRow key={article.id} article={article} deleting={deleting === article.id} publishing={publishing === article.id} onDelete={handleDelete} onPublish={handlePublish} />)}
                  </tbody>
                </table>
              </div>
              <div className="divide-y divide-[var(--admin-border)] md:hidden">
                {articles.items.map((article) => <ArticleMobileCard key={article.id} article={article} deleting={deleting === article.id} publishing={publishing === article.id} onDelete={handleDelete} onPublish={handlePublish} />)}
              </div>
              <AdminPagination page={page} totalPages={totalPages} onPageChange={setPage} />
            </>
          ) : <AdminEmptyState title="暂无文章" description="可以从右上角新建第一篇内容。" />}
        </AdminPanel>
      )}
    </AdminShell>
  )
}

type ManagedArticle = NonNullable<Awaited<ReturnType<typeof getAdminArticleList>>>["items"][number]
type ArticleActions = { article: ManagedArticle; deleting: boolean; publishing: boolean; onDelete: (id: string) => void; onPublish: (id: string) => void }

function ArticleTableRow({ article, deleting, publishing, onDelete, onPublish }: ArticleActions) {
  return (
    <tr className="transition-colors hover:bg-[var(--admin-sky-soft)]/40">
      <td className="px-6 py-4"><Link href={`/editor?id=${article.id}`} className="font-semibold text-[var(--admin-ink)] hover:text-[var(--admin-sky-deep)]">{article.title}</Link></td>
      <td className="px-4 py-4 text-xs text-[var(--admin-muted)]">{article.tags.slice(0, 3).map((tag) => tag.name).join(" · ") || "暂无标签"}</td>
      <td className="px-4 py-4"><AdminStatusBadge status={article.status} /></td>
      <td className="px-4 py-4 text-xs text-[var(--admin-muted)]">{formatNumber(article.views)} / {formatNumber(article.likes)} / {article.commentsCount}</td>
      <td className="px-4 py-4 text-xs text-[var(--admin-muted)]">{formatDate(article.createdAt)}</td>
      <td className="px-4 py-4 text-xs text-[var(--admin-muted)]">{formatDate(article.updatedAt)}</td>
      <td className="px-6 py-4"><ArticleButtons article={article} deleting={deleting} publishing={publishing} onDelete={onDelete} onPublish={onPublish} /></td>
    </tr>
  )
}

function ArticleMobileCard({ article, deleting, publishing, onDelete, onPublish }: ArticleActions) {
  return <article className="p-5"><div className="flex items-start justify-between gap-3"><div><Link href={`/editor?id=${article.id}`} className="font-semibold text-[var(--admin-ink)]">{article.title}</Link><p className="mt-2 text-xs text-[var(--admin-faint)]">{article.tags.slice(0, 3).map((tag) => tag.name).join(" · ") || "暂无标签"}</p><p className="mt-1 text-xs text-[var(--admin-faint)]">创建于 {formatDate(article.createdAt)} · {formatNumber(article.views)} 阅读</p></div><AdminStatusBadge status={article.status} /></div><div className="mt-4"><ArticleButtons article={article} deleting={deleting} publishing={publishing} onDelete={onDelete} onPublish={onPublish} /></div></article>
}

function ArticleButtons({ article, deleting, publishing, onDelete, onPublish }: ArticleActions) {
  return <div className="flex flex-wrap items-center justify-end gap-2">{article.status === "published" ? <Link href={`/articles/${article.id}`}><Button variant="ghost" size="sm" className="rounded-full text-[var(--admin-muted)]"><Eye className="size-3.5" />预览</Button></Link> : null}<Link href={`/editor?id=${article.id}`}><Button variant="outline" size="sm" className="rounded-full border-[var(--admin-border-strong)] bg-transparent"><Edit3 className="size-3.5" />编辑</Button></Link>{article.status === "draft" ? <Button size="sm" onClick={() => onPublish(article.id)} disabled={publishing} className="rounded-full bg-[var(--admin-teal-deep)] text-white"><Send className={cn("size-3.5", publishing && "animate-pulse")} />发布</Button> : null}<Button variant="ghost" size="sm" onClick={() => onDelete(article.id)} disabled={deleting} className="rounded-full text-[var(--admin-coral)] hover:bg-[var(--admin-coral-soft)] hover:text-[var(--admin-coral)]"><Trash2 className={cn("size-3.5", deleting && "animate-pulse")} />删除</Button></div>
}
