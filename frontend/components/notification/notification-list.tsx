"use client"

import { useState } from "react"
import useSWR from "swr"
import { ChevronLeft, ChevronRight } from "lucide-react"
import { getNotificationListKey, getNotifications } from "@/api/notifications"
import { NotificationItem } from "./notification-item"
import { LoadingState, EmptyState } from "@/components/ui/spinner"
import { Button } from "@/components/ui/button"

const PAGE_SIZE = 10

export function NotificationList() {
  const [page, setPage] = useState(1)
  const { data: notifications, isLoading, error } = useSWR(
    getNotificationListKey(page, PAGE_SIZE),
    () => getNotifications(page, PAGE_SIZE),
    { errorRetryCount: 0 },
  )

  if (isLoading && !notifications) return <LoadingState />
  if (error) {
    console.error("Notification list error:", error)
    return <EmptyState label="获取通知失败，请重试" />
  }
  if (!notifications || (notifications.length === 0 && page === 1)) {
    return <EmptyState label="暂无通知" />
  }

  if (notifications.length === 0) {
    return (
      <div className="text-center">
        <EmptyState label="这一页没有更多通知" />
        <Button variant="outline" size="sm" onClick={() => setPage((current) => current - 1)}>
          <ChevronLeft className="size-4" />
          返回上一页
        </Button>
      </div>
    )
  }

  return (
    <>
      <div className="flex flex-col gap-2">
        {notifications.map((notification) => (
          <NotificationItem key={notification.id} notification={notification} />
        ))}
      </div>

      {(page > 1 || notifications.length === PAGE_SIZE) && (
        <nav className="mt-6 flex items-center justify-center gap-3" aria-label="通知分页">
          <Button
            variant="outline"
            size="sm"
            disabled={page <= 1 || isLoading}
            onClick={() => setPage((current) => current - 1)}
          >
            <ChevronLeft className="size-4" />
            上一页
          </Button>
          <span className="text-sm text-muted-foreground">第 {page} 页</span>
          <Button
            variant="outline"
            size="sm"
            disabled={notifications.length < PAGE_SIZE || isLoading}
            onClick={() => setPage((current) => current + 1)}
          >
            下一页
            <ChevronRight className="size-4" />
          </Button>
        </nav>
      )}
    </>
  )
}
