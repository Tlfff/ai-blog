"use client"

import { useState } from "react"
import { ChevronLeft, ChevronRight } from "lucide-react"
import useSWR from "swr"
import { getNotificationListKey, getNotifications } from "@/api/notifications"
import { NotificationItem } from "./notification-item"
import { LoadingState } from "@/components/ui/spinner"
import { Button } from "@/components/ui/button"
import { formatDate } from "@/lib/format"
import type { Notification } from "@/types"

const PAGE_SIZE = 10

export function NotificationList() {
  const [page, setPage] = useState(1)
  const { data: notifications, isLoading, error } = useSWR(getNotificationListKey(page, PAGE_SIZE), () => getNotifications(page, PAGE_SIZE), { errorRetryCount: 0 })
  const groups = groupNotifications(notifications ?? [])

  if (isLoading && !notifications) return <LoadingState label="正在接收通知..." />
  if (error) return <NotificationMessage title="获取通知失败" description="请稍后刷新页面重试。" />
  if (!notifications || (notifications.length === 0 && page === 1)) return <NotificationMessage title="暂无通知" description="新的回复和点赞会从这里抵达。" />
  if (notifications.length === 0) return <div className="pb-8 text-center"><NotificationMessage title="这一页没有更多通知" description="可以返回上一页继续查看。" /><Button variant="outline" size="sm" onClick={() => setPage((current) => current - 1)} className="rounded-full border-[var(--utility-border-strong)] bg-transparent"><ChevronLeft className="size-4" />返回上一页</Button></div>

  return (
    <>
      <div className="px-4 py-5 sm:px-5">
        {groups.map((group) => (
          <section key={group.key} className="mb-7 last:mb-0">
            <h3 className="px-1 font-mono text-[0.62rem] font-semibold uppercase tracking-[0.2em] text-[var(--utility-lavender)]">{group.label}</h3>
            <div className="mt-3 divide-y divide-[var(--utility-border)]">
              {group.items.map((notification) => <NotificationItem key={notification.id} notification={notification} />)}
            </div>
          </section>
        ))}
      </div>
      {(page > 1 || notifications.length === PAGE_SIZE) ? (
        <nav className="flex items-center justify-center gap-3 border-t border-[var(--utility-border)] px-5 py-4" aria-label="通知分页">
          <Button variant="outline" size="sm" disabled={page <= 1 || isLoading} onClick={() => setPage((current) => current - 1)} className="rounded-full border-[var(--utility-border-strong)] bg-transparent"><ChevronLeft className="size-4" />上一页</Button>
          <span className="text-xs text-[var(--utility-muted)]">第 {page} 页</span>
          <Button variant="outline" size="sm" disabled={notifications.length < PAGE_SIZE || isLoading} onClick={() => setPage((current) => current + 1)} className="rounded-full border-[var(--utility-border-strong)] bg-transparent">下一页<ChevronRight className="size-4" /></Button>
        </nav>
      ) : null}
    </>
  )
}

function NotificationMessage({ title, description }: { title: string; description: string }) {
  return <div className="grid min-h-60 place-items-center px-6 py-10 text-center"><div><p className="font-playful text-xl font-bold text-[var(--utility-ink)]">{title}</p><p className="mt-2 text-sm text-[var(--utility-muted)]">{description}</p></div></div>
}

function groupNotifications(items: Notification[]) {
  const groups = new Map<string, { key: string; label: string; items: Notification[] }>()
  items.forEach((item) => {
    const date = new Date(item.createdAt)
    const key = `${date.getFullYear()}-${date.getMonth()}-${date.getDate()}`
    if (!groups.has(key)) groups.set(key, { key, label: getGroupLabel(date), items: [] })
    groups.get(key)?.items.push(item)
  })
  return Array.from(groups.values())
}

function getGroupLabel(date: Date) {
  const now = new Date()
  const today = new Date(now.getFullYear(), now.getMonth(), now.getDate()).getTime()
  const target = new Date(date.getFullYear(), date.getMonth(), date.getDate()).getTime()
  const dayDiff = Math.round((today - target) / 86400000)
  if (dayDiff === 0) return "today / 今天"
  if (dayDiff === 1) return "yesterday / 昨天"
  return `${formatDate(date.toISOString())} / 更早`
}
