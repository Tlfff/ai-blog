import type { Notification } from "@/types"
import { mapBackendNotificationToFrontend } from "@/types"
import { request } from "./client"

export const NOTIFICATION_LIST_KEY = "notifications"
export const NOTIFICATION_UNREAD_COUNT_KEY = "notification-unread-count"

export function getNotificationListKey(page: number, pageSize: number) {
  return [NOTIFICATION_LIST_KEY, page, pageSize] as const
}

export async function getNotifications(page = 1, pageSize = 10): Promise<Notification[]> {
  const params = new URLSearchParams()
  params.set("page", String(page))
  params.set("page_size", String(pageSize))

  const data = await request<{
    list: {
      id: string
      type: number
      is_read: boolean
      created_time: number
      sender_id: number
      sender_nickname: string
      sender_avatar: string
      action_text: string
      article_id?: number
      title?: string
    }[]
    page: number
    page_size: number
  }>(`/auth/ntf/list?${params.toString()}`)

  return data.list.map((item) => mapBackendNotificationToFrontend(item))
}

export async function getUnreadCount(): Promise<number> {
  const data = await request<number>("/auth/ntf/unread-count")
  return data
}

export async function markAllRead(): Promise<void> {
  await request<void>("/auth/ntf/clear-unread", {
    method: "POST",
  })
}
