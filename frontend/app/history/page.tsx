"use client"

import { useState } from "react"
import Link from "next/link"
import { ArrowRight, BookOpen, ChevronLeft, ChevronRight, Clock3, Compass } from "lucide-react"
import useSWR from "swr"
import { getHistory } from "@/api/users"
import { UtilityCenterShell, UtilityPanel } from "@/components/layout/utility-center-shell"
import { Button } from "@/components/ui/button"
import { LoadingState } from "@/components/ui/spinner"
import { useAuth } from "@/hooks/use-auth"
import { formatDate } from "@/lib/format"
import type { HistoryItem } from "@/types"

const TIMELINE_TONES = ["bg-[var(--utility-sky)]", "bg-[var(--utility-teal)]", "bg-[var(--utility-yellow)]", "bg-[var(--utility-lavender)]", "bg-[var(--utility-coral)]"]

export default function HistoryPage() {
  const { isLoggedIn } = useAuth()
  const [page, setPage] = useState(1)
  const { data: history, isLoading } = useSWR(isLoggedIn ? ["history", page] : null, () => getHistory(page))
  const totalPages = history ? Math.max(1, Math.ceil(history.total / history.pageSize)) : 1
  const groups = groupHistoryItems(history?.items ?? [])

  return (
    <UtilityCenterShell
      variant="history"
      eyebrow="reading trail / 浏览历史"
      title="沿着时间，找回读过的片段。"
      description="每一次打开，都为下一次继续阅读留下方向。"
      metricLabel="reading footprint"
      metricValue={isLoggedIn ? `${history?.total ?? "—"} 条记录` : "登录后查看"}
    >
      {!isLoggedIn ? (
        <UtilityPanel className="grid min-h-72 place-items-center p-8 text-center">
          <div><span className="mx-auto grid size-14 place-items-center rounded-2xl bg-[var(--utility-sky-soft)] text-[var(--utility-sky-deep)]"><Clock3 className="size-6" /></span><h2 className="mt-5 font-playful text-2xl font-bold text-[var(--utility-ink)]">登录后查看浏览历史</h2><p className="mt-2 text-sm text-[var(--utility-muted)]">浏览记录会按当前登录用户保存在这台设备上。</p><Link href="/login?redirect=/history" className="mt-6 inline-flex rounded-full bg-[var(--utility-sky-deep)] px-5 py-2.5 text-sm font-semibold text-white">前往登录</Link></div>
        </UtilityPanel>
      ) : (
        <div className="grid gap-5 lg:grid-cols-[minmax(0,1fr)_300px]">
          <UtilityPanel className="overflow-hidden">
            <div className="flex items-start justify-between gap-4 border-b border-[var(--utility-border)] px-5 py-5 sm:px-7">
              <div><h2 className="font-playful text-2xl font-bold text-[var(--utility-ink)]">阅读时间线</h2><p className="mt-1 text-xs text-[var(--utility-faint)]">最近浏览的文章会按时间保存在这里</p></div>
              <span className="hidden font-mono text-[0.58rem] uppercase tracking-[0.16em] text-[var(--utility-faint)] sm:block">page {String(page).padStart(2, "0")} / {String(totalPages).padStart(2, "0")}</span>
            </div>

            {isLoading && !history ? <LoadingState label="正在整理阅读足迹..." /> : groups.length ? (
              <div className="px-5 py-6 sm:px-7">
                {groups.map((group) => (
                  <section key={group.key} className="mb-8 last:mb-0">
                    <h3 className="font-mono text-[0.64rem] font-semibold uppercase tracking-[0.2em] text-[var(--utility-sky-deep)]">{group.label}</h3>
                    <div className="relative mt-4 before:absolute before:bottom-5 before:left-[7px] before:top-4 before:w-px before:bg-[var(--utility-border-strong)]">
                      {group.items.map((item, index) => (
                        <article key={`${item.articleId}-${item.viewedAt}`} className="relative grid gap-2 border-b border-[var(--utility-border)] py-4 pl-8 last:border-b-0 sm:grid-cols-[72px_minmax(0,1fr)_auto] sm:items-center sm:gap-4">
                          <span className={`absolute left-0 top-[1.35rem] z-10 size-3.5 rounded-full border-[3px] border-[var(--utility-card)] ${TIMELINE_TONES[index % TIMELINE_TONES.length]}`} />
                          <time className="text-xs text-[var(--utility-faint)]">{formatHistoryTime(item.viewedAt)}</time>
                          <Link href={`/articles/${item.articleId}`} className="min-w-0 font-semibold text-[var(--utility-ink)] transition-colors hover:text-[var(--utility-sky-deep)]">{item.title}</Link>
                          <Link href={`/articles/${item.articleId}`} className="inline-flex w-fit items-center gap-1 rounded-full bg-[var(--utility-input)] px-3 py-2 text-xs font-semibold text-[var(--utility-muted)] hover:bg-[var(--utility-sky-soft)] hover:text-[var(--utility-sky-deep)]">打开<ArrowRight className="size-3.5" /></Link>
                        </article>
                      ))}
                    </div>
                  </section>
                ))}
              </div>
            ) : (
              <div className="grid min-h-72 place-items-center px-6 py-12 text-center"><div><Clock3 className="mx-auto size-10 text-[var(--utility-faint)]" /><p className="mt-4 font-playful text-xl font-bold text-[var(--utility-ink)]">{page > 1 ? "这一页没有更多记录" : "暂无浏览记录"}</p><p className="mt-2 text-sm text-[var(--utility-muted)]">读过的文章会从这里留下时间足迹。</p></div></div>
            )}

            {history && history.total > history.pageSize ? (
              <nav className="flex items-center justify-center gap-3 border-t border-[var(--utility-border)] px-5 py-4" aria-label="浏览历史分页">
                <Button variant="outline" size="sm" disabled={page <= 1 || isLoading} onClick={() => setPage((current) => current - 1)} className="rounded-full border-[var(--utility-border-strong)] bg-transparent"><ChevronLeft className="size-4" />上一页</Button>
                <span className="text-xs text-[var(--utility-muted)]">第 {page} / {totalPages} 页</span>
                <Button variant="outline" size="sm" disabled={page >= totalPages || isLoading} onClick={() => setPage((current) => current + 1)} className="rounded-full border-[var(--utility-border-strong)] bg-transparent">下一页<ChevronRight className="size-4" /></Button>
              </nav>
            ) : null}
          </UtilityPanel>

          <aside className="space-y-5">
            <UtilityPanel className="overflow-hidden p-5">
              <div className="relative aspect-[16/7] overflow-hidden rounded-2xl bg-[linear-gradient(135deg,#398fc2,#75d1c7)]"><div className="absolute -right-5 top-5 h-14 w-28 rounded-full bg-white/50" /><div className="absolute -left-4 bottom-[-18px] h-16 w-[115%] rounded-[50%] bg-[#226b83]/25" /><p className="absolute bottom-3 left-4 font-mono text-[0.56rem] font-semibold tracking-[0.16em] text-white">KEEP READING / KEEP CURIOUS</p></div>
              <h2 className="mt-5 font-playful text-xl font-bold text-[var(--utility-ink)]">关于浏览记录</h2><p className="mt-3 text-sm leading-7 text-[var(--utility-muted)]">记录保存在当前浏览器中，并按登录用户隔离，方便回到最近读过的文章。</p><p className="mt-4 inline-flex items-center gap-2 rounded-full bg-[var(--utility-teal-soft)] px-3 py-1.5 text-xs font-medium text-[var(--utility-teal-deep)]"><span className="size-2 rounded-full bg-[var(--utility-teal)]" />当前设备已启用</p>
            </UtilityPanel>
            <UtilityPanel className="p-5"><h2 className="font-playful text-xl font-bold text-[var(--utility-ink)]">继续探索</h2><p className="mt-1 text-xs text-[var(--utility-faint)]">从新的内容开始下一次阅读</p><div className="mt-5 space-y-3"><Link href="/#latest" className="flex items-center rounded-xl bg-[var(--utility-sky-soft)] px-4 py-3 text-sm font-semibold text-[var(--utility-sky-deep)]"><BookOpen className="mr-3 size-4" />浏览最新文章<ArrowRight className="ml-auto size-4" /></Link><Link href="/" className="flex items-center rounded-xl bg-[var(--utility-lavender-soft)] px-4 py-3 text-sm font-semibold text-[var(--utility-lavender)]"><Compass className="mr-3 size-4" />返回首页<ArrowRight className="ml-auto size-4" /></Link></div></UtilityPanel>
          </aside>
        </div>
      )}
    </UtilityCenterShell>
  )
}

function groupHistoryItems(items: HistoryItem[]) {
  const groups = new Map<string, { key: string; label: string; items: HistoryItem[] }>()
  items.forEach((item) => {
    const date = new Date(item.viewedAt)
    const key = `${date.getFullYear()}-${date.getMonth()}-${date.getDate()}`
    if (!groups.has(key)) groups.set(key, { key, label: getHistoryGroupLabel(date), items: [] })
    groups.get(key)?.items.push(item)
  })
  return Array.from(groups.values())
}

function getHistoryGroupLabel(date: Date) {
  const now = new Date()
  const today = new Date(now.getFullYear(), now.getMonth(), now.getDate()).getTime()
  const target = new Date(date.getFullYear(), date.getMonth(), date.getDate()).getTime()
  const dayDiff = Math.round((today - target) / 86400000)
  if (dayDiff === 0) return "today / 今天"
  if (dayDiff === 1) return "yesterday / 昨天"
  return `${formatDate(date.toISOString())} / 更早`
}

function formatHistoryTime(value: string) {
  return new Intl.DateTimeFormat("zh-CN", { hour: "2-digit", minute: "2-digit", hour12: false }).format(new Date(value))
}
