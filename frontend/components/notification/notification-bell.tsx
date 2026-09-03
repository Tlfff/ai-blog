"use client"

import Link from "next/link"
import { Bell } from "lucide-react"
import useSWR from "swr"
import { getUnreadCount, NOTIFICATION_UNREAD_COUNT_KEY } from "@/api/notifications"
import { useAuth } from "@/hooks/use-auth"

export function NotificationBell() {
  const { isLoggedIn } = useAuth()
  const { data: count = 0 } = useSWR(
    isLoggedIn ? NOTIFICATION_UNREAD_COUNT_KEY : null,
    getUnreadCount,
  )

  if (!isLoggedIn) return null

  return (
    <Link
      href="/notifications"
      className="relative inline-flex size-8 items-center justify-center rounded-lg text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
      aria-label={`通知中心，${count} 条未读`}
    >
      <Bell className="size-[18px]" />
      {count > 0 && (
        <span className="absolute -right-0.5 -top-0.5 inline-flex min-w-4 items-center justify-center rounded-full bg-destructive px-1 text-[10px] font-medium leading-4 text-destructive-foreground">
          {count > 99 ? "99+" : count}
        </span>
      )}
    </Link>
  )
}
