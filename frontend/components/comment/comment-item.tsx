"use client"

import { useState } from "react"
import Link from "next/link"
import useSWR from "swr"
import { ChevronDown, ChevronUp, Reply, ThumbsUp, Trash2 } from "lucide-react"
import { Avatar } from "@/components/ui/avatar"
import { Button } from "@/components/ui/button"
import { formatRelativeTime } from "@/lib/format"
import { cn } from "@/lib/utils"
import type { Comment } from "@/types"
import { deleteComment, getReplies, toggleCommentLike } from "@/api/comments"
import { CommentForm } from "./comment-form"
import { useAuth } from "@/hooks/use-auth"

interface CommentItemProps {
  comment: Comment
  articleId: string
  articleAuthorId?: string
  onChanged?: () => void
}

export function CommentItem({ comment, articleId, articleAuthorId, onChanged }: CommentItemProps) {
  const { user, isLoggedIn } = useAuth()
  const [expanded, setExpanded] = useState(false)
  const [replying, setReplying] = useState(false)
  const [deleting, setDeleting] = useState(false)
  const [likes, setLikes] = useState(comment.likes)
  const [liked, setLiked] = useState(comment.liked ?? false)

  const { data: replies, isLoading: repliesLoading, mutate: mutateReplies } = useSWR(
    expanded && comment.parentId === null ? ["replies", comment.id] : null,
    () => getReplies(comment.id),
  )

  async function handleLike() {
    if (!isLoggedIn) return
    await toggleCommentLike(comment.id, liked)
    const newLiked = !liked
    setLiked(newLiked)
    setLikes(likes + (newLiked ? 1 : -1))
  }

  async function handleDelete() {
    if (!user || user.id !== comment.author.id || !confirm("确定要删除这条评论吗？")) return
    setDeleting(true)
    try {
      await deleteComment(comment.id)
      onChanged?.()
    } finally {
      setDeleting(false)
    }
  }

  const isArticleAuthor = comment.author.id === articleAuthorId
  const rootId = comment.parentId ?? comment.id

  if (comment.deleted) {
    return (
      <div className="border-b border-border py-4 text-sm text-muted-foreground">
        {comment.content}
      </div>
    )
  }

  return (
    <div className="border-b border-border py-4">
      <div className="flex gap-3">
        <Link href={`/profile?userId=${comment.author.id}`} className="shrink-0">
          <Avatar src={comment.author.avatar} alt={comment.author.username} size={40} />
        </Link>

        <div className="flex flex-1 flex-col">
          <div className="flex flex-wrap items-center gap-2">
            <Link href={`/profile?userId=${comment.author.id}`} className="font-medium hover:text-primary">
              {comment.author.username}
            </Link>
            {isArticleAuthor && <span className="rounded-full bg-primary/10 px-2 py-0.5 text-xs text-primary">作者</span>}
            {comment.ip && <span className="text-xs text-muted-foreground/70">{comment.ip}</span>}
            <span className="text-sm text-muted-foreground">{formatRelativeTime(comment.createdAt)}</span>
          </div>

          <div className="mt-1.5 text-sm leading-relaxed">
            {comment.replyToUser && (
              <span className="text-muted-foreground">
                @{comment.replyToUser.username}:{" "}
              </span>
            )}
            {comment.content}
          </div>

          <div className="mt-2 flex items-center gap-4">
            <Button
              variant="ghost"
              size="icon-xs"
              onClick={handleLike}
              disabled={!isLoggedIn}
              className={cn(liked && "text-primary")}
              aria-label={liked ? "取消点赞" : "点赞"}
            >
              <ThumbsUp className={cn("size-3.5", liked && "fill-primary")} />
              <span className="ml-1 text-xs">{likes}</span>
            </Button>

            <Button
              variant="ghost"
              size="icon-xs"
              onClick={() => setReplying(!replying)}
              disabled={!isLoggedIn}
              className="text-muted-foreground hover:text-foreground"
              aria-label="回复"
            >
              <Reply className="size-3.5" />
              <span className="ml-1 text-xs">回复</span>
            </Button>

            {comment.parentId === null && comment.replyCount && comment.replyCount > 0 ? (
              <Button
                variant="ghost"
                size="icon-xs"
                onClick={() => setExpanded(!expanded)}
                className="text-muted-foreground hover:text-foreground"
              >
                {expanded ? <ChevronUp className="size-3.5" /> : <ChevronDown className="size-3.5" />}
                <span className="ml-1 text-xs">{comment.replyCount} 条回复</span>
              </Button>
            ) : null}

            {user?.id === comment.author.id && (
              <Button
                variant="ghost"
                size="icon-xs"
                onClick={handleDelete}
                disabled={deleting}
                className="text-muted-foreground hover:text-destructive"
                aria-label="删除评论"
              >
                <Trash2 className="size-3.5" />
                <span className="ml-1 text-xs">{deleting ? "删除中" : "删除"}</span>
              </Button>
            )}
          </div>

          {replying && (
            <div className="mt-3">
              <CommentForm
                articleId={articleId}
                parentId={rootId}
                replyToUser={{ id: comment.author.id, username: comment.author.username }}
                onCancel={() => setReplying(false)}
                onSubmit={() => {
                  setReplying(false)
                  if (comment.parentId === null) setExpanded(true)
                  void mutateReplies()
                  onChanged?.()
                }}
              />
            </div>
          )}

          {expanded && (
            <div className="mt-4 ml-4 border-l border-border pl-4">
              {repliesLoading ? (
                <div className="py-2 text-sm text-muted-foreground">加载中...</div>
              ) : replies && replies.length > 0 ? (
                replies.map((reply) => (
                  <CommentItem
                    key={reply.id}
                    comment={reply}
                    articleId={articleId}
                    articleAuthorId={articleAuthorId}
                    onChanged={() => {
                      void mutateReplies()
                      onChanged?.()
                    }}
                  />
                ))
              ) : (
                <div className="py-2 text-sm text-muted-foreground">暂无回复</div>
              )}
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
