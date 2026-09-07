"use client"

import { SITE_IMAGES } from "@/lib/site-images"
import Image from "next/image"
import Link from "next/link"
import { BookOpen, FileText, MessageSquare, PenLine, Trash2 } from "lucide-react"
import useSWR from "swr"
import { getAdminArticleList } from "@/api/articles"
import { getAdminCommentCollection } from "@/api/comments"
import { AdminShell } from "@/components/admin/admin-shell"
import { AdminPanel, AdminPanelHeading, AdminStatusBadge } from "@/components/admin/admin-ui"
import { useAuth } from "@/hooks/use-auth"
import { formatDate } from "@/lib/format"

const STAT_TONES = {
  sky: "bg-[var(--admin-sky-soft)] text-[var(--admin-sky-deep)]",
  yellow: "bg-[var(--admin-yellow-soft)] text-[var(--admin-yellow-deep)]",
  teal: "bg-[var(--admin-teal-soft)] text-[var(--admin-teal-deep)]",
  coral: "bg-[var(--admin-coral-soft)] text-[var(--admin-coral)]",
} as const

export default function AdminPage() {
  const { user, isAdmin } = useAuth()
  const { data: publishedArticles } = useSWR(isAdmin ? "admin-published" : null, () => getAdminArticleList(3))
  const { data: draftArticles } = useSWR(isAdmin ? "admin-drafts" : null, () => getAdminArticleList(2))
  const { data: deletedArticles } = useSWR(isAdmin ? "admin-deleted" : null, () => getAdminArticleList(1))
  const { data: commentCollection } = useSWR(isAdmin ? "admin-comment-collection" : null, getAdminCommentCollection)

  const recentArticles = [...(publishedArticles?.items ?? []), ...(draftArticles?.items ?? [])]
    .sort((a, b) => new Date(b.updatedAt).getTime() - new Date(a.updatedAt).getTime())
    .slice(0, 4)
  const articleMap = new Map(commentCollection?.articles.map((article) => [article.id, article.title]) ?? [])
  const recentComments = (commentCollection?.comments ?? []).slice(0, 3)
  const stats = [
    { label: "已发表", value: publishedArticles?.total, unit: "篇文章", icon: BookOpen, tone: "sky" },
    { label: "草稿箱", value: draftArticles?.total, unit: "篇待完成", icon: FileText, tone: "yellow" },
    { label: "评论", value: commentCollection?.comments.length, unit: "条互动", icon: MessageSquare, tone: "teal" },
    { label: "回收站", value: deletedArticles?.total, unit: "篇已删除", icon: Trash2, tone: "coral" },
  ] as const

  return (
    <AdminShell>
      <section className="relative min-h-44 overflow-hidden rounded-[1.35rem] bg-[#2678ad] text-white shadow-[0_18px_45px_var(--admin-shadow)]">
        <Image src={SITE_IMAGES.pages.primarySky} alt="蓝天与云朵" fill priority sizes="(min-width: 1024px) calc(100vw - 322px), 100vw" className="object-cover object-[center_22%]" />
        <div className="absolute inset-0 bg-[linear-gradient(90deg,rgba(19,79,119,0.88),rgba(29,137,173,0.52),rgba(57,176,177,0.38))]" />
        <div className="relative flex min-h-44 flex-col items-start justify-center p-6 sm:p-8">
          <p className="font-mono text-[0.6rem] font-semibold uppercase tracking-[0.22em] text-white/80">today / keep the notes in order</p>
          <h1 className="mt-3 font-playful text-3xl font-bold sm:text-4xl">下午好，{user?.username ?? "管理员"}。</h1>
          <p className="mt-2 text-sm text-white/86">今天也把内容整理得井井有条。</p>
          <div className="mt-5 flex gap-2">
            <Link href="/editor" className="inline-flex items-center gap-2 rounded-full bg-white px-4 py-2 text-sm font-bold text-[#2877a5] hover:bg-white/90"><PenLine className="size-4" />写一篇文章</Link>
            <Link href="/" className="rounded-full border border-white/45 bg-[#143b57]/25 px-4 py-2 text-sm font-semibold text-white backdrop-blur-sm hover:bg-[#143b57]/40">查看博客</Link>
          </div>
        </div>
      </section>

      <section className="mt-5 grid grid-cols-2 gap-3 xl:grid-cols-4">
        {stats.map((stat) => {
          const Icon = stat.icon
          return (
            <AdminPanel key={stat.label} className="flex min-h-28 items-center gap-4 p-4 sm:p-5">
              <span className={`grid size-11 shrink-0 place-items-center rounded-2xl ${STAT_TONES[stat.tone]}`}><Icon className="size-5" /></span>
              <div>
                <p className="text-xs text-[var(--admin-muted)]">{stat.label}</p>
                <div className="mt-1 flex items-end gap-1.5"><strong className="font-playful text-3xl text-[var(--admin-ink)]">{stat.value ?? "—"}</strong><span className="pb-1 text-[0.68rem] text-[var(--admin-faint)]">{stat.unit}</span></div>
              </div>
            </AdminPanel>
          )
        })}
      </section>

      <section className="mt-5 grid gap-5 xl:grid-cols-[minmax(0,1.9fr)_minmax(320px,0.95fr)]">
        <AdminPanel>
          <AdminPanelHeading title="最近文章" description="继续编辑或查看最近更新的内容" action={<Link href="/admin/articles" className="text-xs font-semibold text-[var(--admin-sky-deep)]">查看全部 →</Link>} />
          <div className="divide-y divide-[var(--admin-border)] px-5 sm:px-6">
            {recentArticles.map((article) => (
              <div key={article.id} className="grid gap-3 py-4 sm:grid-cols-[minmax(0,1fr)_auto_auto] sm:items-center">
                <div className="min-w-0">
                  <Link href={`/editor?id=${article.id}`} className="line-clamp-1 text-sm font-bold text-[var(--admin-ink)] hover:text-[var(--admin-sky-deep)]">{article.title}</Link>
                  <p className="mt-1 text-xs text-[var(--admin-faint)]">{article.tags.slice(0, 2).map((tag) => tag.name).join(" · ") || "暂无标签"}</p>
                </div>
                <div className="flex items-center gap-3"><AdminStatusBadge status={article.status} /><span className="text-xs text-[var(--admin-faint)]">{formatDate(article.updatedAt)}</span></div>
                <Link href={`/editor?id=${article.id}`} className="w-fit rounded-full bg-[var(--admin-sky-soft)] px-3 py-1.5 text-xs font-semibold text-[var(--admin-sky-deep)]">{article.status === "draft" ? "继续写" : "编辑"}</Link>
              </div>
            ))}
            {recentArticles.length === 0 ? <p className="py-12 text-center text-sm text-[var(--admin-muted)]">还没有文章，先写下第一篇吧。</p> : null}
          </div>
          <div className="m-5 flex items-center gap-3 rounded-xl bg-[var(--admin-input)] px-4 py-3 sm:m-6">
            <span className="grid size-6 place-items-center rounded-full bg-[var(--admin-sky-soft)] text-[var(--admin-sky-deep)]">+</span>
            <div><p className="text-xs font-semibold text-[var(--admin-ink)]">新文章从清晰的标题开始</p><p className="mt-0.5 text-[0.68rem] text-[var(--admin-faint)]">进入编辑器，正文仍保存为 Markdown。</p></div>
            <Link href="/editor" className="ml-auto shrink-0 text-xs font-semibold text-[var(--admin-sky-deep)]">开始写作 →</Link>
          </div>
        </AdminPanel>

        <div className="space-y-5">
          <AdminPanel>
            <AdminPanelHeading title="最近评论" description="查看站内最新互动" action={<Link href="/admin/comments" className="text-xs font-semibold text-[var(--admin-sky-deep)]">评论管理 →</Link>} />
            <div className="divide-y divide-[var(--admin-border)] px-5 sm:px-6">
              {recentComments.map((comment) => (
                <div key={comment.id} className="flex gap-3 py-4">
                  <span className="grid size-8 shrink-0 place-items-center rounded-full bg-[var(--admin-teal-soft)] text-xs font-bold text-[var(--admin-teal-deep)]">{comment.author.username.slice(0, 1)}</span>
                  <div className="min-w-0"><p className="text-xs font-bold text-[var(--admin-ink)]">{comment.author.username}</p><p className="mt-1 line-clamp-2 text-xs leading-5 text-[var(--admin-muted)]">{comment.content}</p><p className="mt-1 truncate text-[0.65rem] text-[var(--admin-faint)]">{articleMap.get(comment.articleId) ?? "未知文章"}</p></div>
                </div>
              ))}
              {recentComments.length === 0 ? <p className="py-10 text-center text-sm text-[var(--admin-muted)]">暂无评论</p> : null}
            </div>
          </AdminPanel>

          <AdminPanel className="p-5 sm:p-6">
            <h2 className="font-playful text-xl font-bold text-[var(--admin-ink)]">内容状态</h2>
            <p className="mt-1 text-xs text-[var(--admin-faint)]">需要留意的内容节点</p>
            <Link href="/admin/drafts" className="mt-5 flex items-center rounded-xl bg-[var(--admin-yellow-soft)] px-4 py-3 text-xs font-semibold text-[var(--admin-yellow-deep)]"><span className="mr-3 size-2 rounded-full bg-[var(--admin-yellow)]" />{draftArticles?.total ?? "—"} 篇草稿等待完成<span className="ml-auto">去处理 →</span></Link>
            <Link href="/admin/trash" className="mt-3 flex items-center rounded-xl bg-[var(--admin-coral-soft)] px-4 py-3 text-xs font-semibold text-[var(--admin-coral)]"><span className="mr-3 size-2 rounded-full bg-[var(--admin-coral)]" />{deletedArticles?.total ?? "—"} 篇文章位于回收站<span className="ml-auto">查看 →</span></Link>
          </AdminPanel>
        </div>
      </section>
    </AdminShell>
  )
}
