"use client"

import Link from "next/link"
import { ArrowRight, Heart, MessageCircle, MessageSquare } from "lucide-react"
import { Avatar } from "@/components/ui/avatar"
import { cn } from "@/lib/utils"
import { formatRelativeTime } from "@/lib/format"
import type { Notification } from "@/types"

const TYPE_CONFIG = {
  article_like: {
    title: "点赞了你的文章",
    action: "打开文章",
    icon: Heart,
    tone: "bg-[var(--utility-coral-soft)] text-[var(--utility-coral)]",
  },
  comment_reply: {
    title: "回复了你的评论",
    action: "查看回复",
    icon: MessageCircle,
    tone: "bg-[var(--utility-lavender-soft)] text-[var(--utility-lavender)]",
  },
  comment_like: {
    title: "点赞了你的评论",
    action: "查看评论",
    icon: MessageSquare,
    tone: "bg-[var(--utility-sky-soft)] text-[var(--utility-sky-deep)]",
  },
} as const

export function NotificationItem({ notification }: { notification: Notification }) {
  const config = TYPE_CONFIG[notification.type]
  const Icon = config.icon
  const target = notification.articleId ? `/articles/${notification.articleId}` : "/notifications"

  return (
    <article className={cn("relative flex gap-3 rounded-2xl px-3 py-4 transition-colors sm:gap-4 sm:px-4", notification.read ? "hover:bg-[var(--utility-input)]" : "bg-[var(--utility-lavender-soft)]/70 before:absolute before:bottom-3 before:left-0 before:top-3 before:w-1 before:rounded-full before:bg-[var(--utility-lavender)]")}>
      <div className="relative shrink-0">
        <Avatar src={notification.actor.avatar} alt={notification.actor.username} size={46} className="border-2 border-[var(--utility-card)]" />
        {!notification.read ? <span className="absolute -right-0.5 -top-0.5 size-3 rounded-full border-2 border-[var(--utility-card)] bg-[var(--utility-coral)]" /> : null}
      </div>

      <div className="min-w-0 flex-1">
        <div className="flex flex-wrap items-center gap-2">
          <span className={cn("grid size-7 place-items-center rounded-full", config.tone)}><Icon className="size-3.5" /></span>
          <h4 className="text-sm font-bold text-[var(--utility-ink)]">{notification.actor.username} {config.title}</h4>
          <time className="ml-auto text-[0.68rem] text-[var(--utility-faint)]">{formatRelativeTime(notification.createdAt)}</time>
        </div>

        {notification.actionText ? <p className="mt-2 text-sm leading-6 text-[var(--utility-muted)]">{notification.actionText}</p> : null}
        {notification.articleTitle ? <Link href={target} className="mt-1.5 block truncate text-xs font-medium text-[var(--utility-teal-deep)] hover:underline">{notification.articleTitle}</Link> : null}
        <Link href={target} className="mt-3 inline-flex items-center gap-1 text-xs font-semibold text-[var(--utility-sky-deep)]">{config.action}<ArrowRight className="size-3.5" /></Link>
      </div>
    </article>
  )
}
