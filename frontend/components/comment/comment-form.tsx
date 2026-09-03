"use client"

import { useState } from "react"
import { Send } from "lucide-react"
import { Button } from "@/components/ui/button"
import { createComment } from "@/api/comments"
import { useAuth } from "@/hooks/use-auth"

interface CommentFormProps {
  articleId?: string
  parentId?: string | null
  replyToUser?: { id: string; username: string }
  onCancel?: () => void
  onSubmit?: () => void
}

export function CommentForm({ articleId, parentId, replyToUser, onCancel, onSubmit }: CommentFormProps) {
  const { user, isLoggedIn } = useAuth()
  const [content, setContent] = useState("")
  const [submitting, setSubmitting] = useState(false)

  if (!isLoggedIn) {
    return (
      <div className="rounded-lg border border-border p-4 text-center text-sm text-muted-foreground">
        请先登录后再评论
      </div>
    )
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    const text = content.trim()
    if (!text || !user || !articleId) return

    setSubmitting(true)
    try {
      await createComment({
        articleId,
        content: text,
        rootId: parentId ? Number(parentId) : undefined,
        replyToUserId: replyToUser ? Number(replyToUser.id) : undefined,
      })
      setContent("")
      onSubmit?.()
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <form onSubmit={handleSubmit} className="flex gap-2">
      <input
        type="text"
        value={content}
        onChange={(e) => setContent(e.target.value)}
        placeholder={replyToUser ? `回复 @${replyToUser.username}` : "写下你的评论..."}
        className="flex-1 rounded-lg border border-border px-3 py-2 text-sm outline-none focus:border-ring focus:ring-2 focus:ring-ring/40"
        disabled={submitting}
      />
      <Button type="submit" size="sm" disabled={!content.trim() || submitting}>
        <Send className="size-4" />
        {submitting ? "发送中" : "发送"}
      </Button>
      {onCancel && (
        <Button type="button" variant="ghost" size="sm" onClick={onCancel}>
          取消
        </Button>
      )}
    </form>
  )
}