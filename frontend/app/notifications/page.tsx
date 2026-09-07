"use client"

import { useState } from "react"
import Link from "next/link"
import { Bell, CheckCheck, Heart, MessageCircle, MessageSquare } from "lucide-react"
import useSWR, { mutate } from "swr"
import { getUnreadCount, markAllRead, NOTIFICATION_LIST_KEY, NOTIFICATION_UNREAD_COUNT_KEY } from "@/api/notifications"
import { UtilityCenterShell, UtilityPanel } from "@/components/layout/utility-center-shell"
import { NotificationList } from "@/components/notification/notification-list"
import { Button } from "@/components/ui/button"
import { useAuth } from "@/hooks/use-auth"

export default function NotificationsPage() {
  const { isLoggedIn } = useAuth()
  const [marking, setMarking] = useState(false)
  const { data: unreadCount = 0 } = useSWR(isLoggedIn ? NOTIFICATION_UNREAD_COUNT_KEY : null, getUnreadCount)

  async function handleMarkAllRead() {
    setMarking(true)
    try {
      await markAllRead()
      await Promise.all([
        mutate((key) => Array.isArray(key) && key[0] === NOTIFICATION_LIST_KEY, undefined, { revalidate: true }),
        mutate(NOTIFICATION_UNREAD_COUNT_KEY, 0, { revalidate: false }),
      ])
    } finally {
      setMarking(false)
    }
  }

  return (
    <UtilityCenterShell
      variant="notifications"
      eyebrow="community signal / 通知中心"
      title="每一次回应，都在这里抵达。"
      description="回复、点赞与新的互动，按时间安静地排列。"
      metricLabel="unread signal"
      metricValue={isLoggedIn ? `${unreadCount} 条未读` : "登录后查看"}
    >
      {!isLoggedIn ? (
        <UtilityPanel className="grid min-h-72 place-items-center p-8 text-center">
          <div><span className="mx-auto grid size-14 place-items-center rounded-2xl bg-[var(--utility-lavender-soft)] text-[var(--utility-lavender)]"><Bell className="size-6" /></span><h2 className="mt-5 font-playful text-2xl font-bold text-[var(--utility-ink)]">登录后查看通知</h2><p className="mt-2 text-sm text-[var(--utility-muted)]">文章和评论产生的新互动会集中显示在这里。</p><Link href="/login?redirect=/notifications" className="mt-6 inline-flex rounded-full bg-[var(--utility-lavender)] px-5 py-2.5 text-sm font-semibold text-white">前往登录</Link></div>
        </UtilityPanel>
      ) : (
        <div className="grid gap-5 lg:grid-cols-[minmax(0,1fr)_314px]">
          <UtilityPanel className="overflow-hidden">
            <div className="flex flex-col gap-4 border-b border-[var(--utility-border)] px-5 py-5 sm:flex-row sm:items-center sm:justify-between sm:px-7">
              <div><h2 className="font-playful text-2xl font-bold text-[var(--utility-ink)]">消息列表</h2><p className="mt-1 text-xs text-[var(--utility-faint)]">按时间查看站内互动</p></div>
              <Button variant="outline" size="sm" onClick={handleMarkAllRead} disabled={marking || unreadCount === 0} className="w-fit rounded-full border-0 bg-[var(--utility-lavender-soft)] text-[var(--utility-lavender)] hover:bg-[var(--utility-lavender-soft)] hover:text-[var(--utility-lavender)]">
                <CheckCheck className="size-4" />{marking ? "标记中..." : unreadCount === 0 ? "已全部阅读" : "全部标为已读"}
              </Button>
            </div>
            <NotificationList />
          </UtilityPanel>

          <aside className="space-y-5">
            <UtilityPanel className="p-5">
              <div className="rounded-2xl bg-[linear-gradient(135deg,#f09b99,#dfa2cc_55%,#aaa1e3)] p-5 text-white"><div className="flex items-center gap-3"><span className="grid size-11 place-items-center rounded-full bg-white/18"><Bell className="size-5" /></span><div><p className="font-mono text-[0.58rem] uppercase tracking-[0.16em] text-white/76">unread signal</p><p className="mt-1 font-playful text-2xl font-bold">{unreadCount} 条未读</p></div></div></div>
              <h2 className="mt-5 font-playful text-xl font-bold text-[var(--utility-ink)]">通知说明</h2><p className="mt-3 text-sm leading-7 text-[var(--utility-muted)]">回复和点赞会出现在这里，未读消息使用彩色圆点和浅色背景标记。</p><p className="mt-4 inline-flex items-center gap-2 rounded-full bg-[var(--utility-teal-soft)] px-3 py-1.5 text-xs font-medium text-[var(--utility-teal-deep)]"><span className="size-2 rounded-full bg-[var(--utility-teal)]" />通知服务运行正常</p>
            </UtilityPanel>
            <UtilityPanel className="p-5"><h2 className="font-playful text-xl font-bold text-[var(--utility-ink)]">三种互动</h2><p className="mt-1 text-xs text-[var(--utility-faint)]">保持简单，不增加额外分类操作</p><div className="mt-5 space-y-4 border-t border-[var(--utility-border)] pt-5"><NoticeType icon={<MessageCircle className="size-4" />} tone="lavender" label="评论回复" /><NoticeType icon={<Heart className="size-4" />} tone="coral" label="文章点赞" /><NoticeType icon={<MessageSquare className="size-4" />} tone="sky" label="评论点赞" /></div></UtilityPanel>
          </aside>
        </div>
      )}
    </UtilityCenterShell>
  )
}

function NoticeType({ icon, tone, label }: { icon: React.ReactNode; tone: "lavender" | "coral" | "sky"; label: string }) {
  const toneClass = { lavender: "bg-[var(--utility-lavender-soft)] text-[var(--utility-lavender)]", coral: "bg-[var(--utility-coral-soft)] text-[var(--utility-coral)]", sky: "bg-[var(--utility-sky-soft)] text-[var(--utility-sky-deep)]" }[tone]
  return <div className="flex items-center gap-3 text-sm font-semibold text-[var(--utility-muted)]"><span className={`grid size-8 place-items-center rounded-full ${toneClass}`}>{icon}</span>{label}</div>
}
