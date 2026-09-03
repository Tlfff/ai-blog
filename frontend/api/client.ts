import type { Paginated, HistoryItem } from "@/types"

export const API_BASE_URL = process.env.NEXT_PUBLIC_API_BASE_URL ?? "/api"

const UNAUTHORIZED_CODES = [1002, 1200, 1201]

function handleUnauthorized() {
  localStorage.removeItem("access_token")
  if (typeof window !== "undefined") {
    window.location.href = "/"
  }
}

export async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const token = typeof window !== "undefined" ? localStorage.getItem("access_token") : null
  const headers: HeadersInit = {
    "Content-Type": "application/json",
    ...(token && { Authorization: `Bearer ${token}` }),
    ...init?.headers,
  }

  const res = await fetch(`${API_BASE_URL}${path}`, {
    headers,
    ...init,
  })

  const data = await res.json()

  if (!data.success) {
    if (UNAUTHORIZED_CODES.includes(data.code)) {
      handleUnauthorized()
    }
    throw new Error(data.message || `请求失败: ${data.code}`)
  }

  return data.data as T
}

export function getUserIdFromToken(): string {
  if (typeof window === "undefined") return "guest"
  const token = localStorage.getItem("access_token")
  if (!token) return "guest"
  try {
    if (token.includes(".")) {
      const payload = token.split(".")[1]
      const decoded = JSON.parse(atob(payload))
      return String(decoded.user_id || "guest")
    }
    return token.slice(0, 10)
  } catch {
    return "guest"
  }
}

export function saveToLocalHistory(articleId: string, title: string) {
  if (typeof window === "undefined") return
  try {
    const userId = getUserIdFromToken()
    const key = `view_history_${userId}`
    const raw = localStorage.getItem(key) || "[]"
    const history: any[] = JSON.parse(raw)
    const filtered = history.filter((item: any) => item.articleId !== articleId)
    filtered.unshift({
      articleId,
      title,
      viewedAt: new Date().toISOString(),
    })
    const limited = filtered.slice(0, 100)
    localStorage.setItem(key, JSON.stringify(limited))
  } catch (e) {
    console.error("saveToLocalHistory error:", e)
  }
}

export function getLocalHistory(page = 1, pageSize = 10): Paginated<HistoryItem> {
  if (typeof window === "undefined") {
    return { items: [], total: 0, page, pageSize }
  }
  try {
    const userId = getUserIdFromToken()
    const key = `view_history_${userId}`
    const raw = localStorage.getItem(key) || "[]"
    const history: HistoryItem[] = JSON.parse(raw)
    const start = (page - 1) * pageSize
    const end = page * pageSize
    return {
      items: history.slice(start, end),
      total: history.length,
      page,
      pageSize,
    }
  } catch (e) {
    return { items: [], total: 0, page, pageSize }
  }
}
