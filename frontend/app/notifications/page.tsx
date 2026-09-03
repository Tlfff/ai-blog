"use client"

import { useState } from "react"
import { mutate } from "swr"
import { Bell, BellOff } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Container } from "@/components/layout/container"
import { SiteShell } from "@/components/layout/site-shell"
import { NotificationList } from "@/components/notification/notification-list"
import {
  markAllRead,
  NOTIFICATION_LIST_KEY,
  NOTIFICATION_UNREAD_COUNT_KEY,
} from "@/api/notifications"
import { useAuth } from "@/hooks/use-auth"

export default function NotificationsPage() {
  const { isLoggedIn } = useAuth()
  const [marking, setMarking] = useState(false)

  async function handleMarkAllRead() {
    setMarking(true)
    try {
      await markAllRead()
      await Promise.all([
        mutate(
          (key) => Array.isArray(key) && key[0] === NOTIFICATION_LIST_KEY,
          undefined,
          { revalidate: true },
        ),
        mutate(NOTIFICATION_UNREAD_COUNT_KEY, 0, { revalidate: false }),
      ])
    } finally {
      setMarking(false)
    }
  }

  if (!isLoggedIn) {
    return (
      <SiteShell>
        <Container className="py-12 text-center">
          <BellOff className="mx-auto mb-4 size-12 text-muted-foreground" />
          <p className="text-muted-foreground">请先登录查看通知</p>
        </Container>
      </SiteShell>
    )
  }

  return (
    <SiteShell>
      <Container className="py-6">
        <div className="mx-auto max-w-2xl">
          <div className="mb-6 flex items-center justify-between">
            <h1 className="flex items-center gap-2 text-xl font-semibold">
              <Bell className="size-5" />
              通知中心
            </h1>
            <Button variant="outline" size="sm" onClick={handleMarkAllRead} disabled={marking}>
              {marking ? "标记中..." : "全部已读"}
            </Button>
          </div>

          <NotificationList />
        </div>
      </Container>
    </SiteShell>
  )
}
