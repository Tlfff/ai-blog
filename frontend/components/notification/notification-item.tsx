"use client"

import Link from "next/link"
import { Heart, MessageSquare, MessageCircle } from "lucide-react"
import { Avatar } from "@/components/ui/avatar"
import { cn } from "@/lib/utils"
import { formatRelativeTime } from "@/lib/format"
import type { Notification } from "@/types"

interface NotificationItemProps {
  notification: Notification
}

export function NotificationItem({ notification }: NotificationItemProps) {
  const getIcon = () => {
    switch (notification.type) {
      case "article_like":
        return <Heart className="size-4 text-destructive" />
      case "comment_reply":
        return <MessageCircle className="size-4 text-primary" />
      case "comment_like":
        return <MessageSquare className="size-4 text-secondary" />
    }
  }

  const getText = () => {
    switch (notification.type) {
      case "article_like":
        return (
          <span>
            {notification.actionText}{" "}
            {notification.articleTitle && (
              <Link href={`/articles/${notification.articleId}`} className="font-medium hover:text-primary">
                {notification.articleTitle}
              </Link>
            )}
          </span>
        )
      case "comment_reply":
      case "comment_like":
        return (
          <span>
            {notification.actionText}
          </span>
        )
    }
  }

  return (
    <div
      className={cn(
        "flex gap-3 rounded-lg p-3 transition-colors",
        notification.read ? "hover:bg-muted" : "bg-muted/50",
      )}
    >
      <span className="shrink-0">
        <Avatar src={notification.actor.avatar} alt={notification.actor.username} size={40} />
      </span>

      <div className="flex flex-1 flex-col">
        <div className="flex items-start gap-2">
          <span className="font-medium">
            {notification.actor.username}
          </span>
          {getIcon()}
        </div>

        <div className="mt-0.5 text-sm">{getText()}</div>

        <span className="mt-1 text-xs text-muted-foreground">
          {formatRelativeTime(notification.createdAt)}
        </span>
      </div>

      {!notification.read && (
        <span className="mt-2 flex h-2 w-2 shrink-0 rounded-full bg-primary" />
      )}
    </div>
  )
}
