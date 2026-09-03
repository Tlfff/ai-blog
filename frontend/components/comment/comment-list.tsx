"use client"

import { useState } from "react"
import useSWR from "swr"
import { MessageSquare, Clock, Filter, ChevronLeft, ChevronRight } from "lucide-react"
import { getComments } from "@/api/comments"
import { CommentForm } from "./comment-form"
import { CommentItem } from "./comment-item"
import { LoadingState, EmptyState } from "@/components/ui/spinner"
import { Button } from "@/components/ui/button"
import { cn } from "@/lib/utils"
import type { CommentSort } from "@/types"

interface CommentListProps {
  articleId: string
  authorId?: string
}

export function CommentList({ articleId, authorId }: CommentListProps) {
  const [sort, setSort] = useState<CommentSort>("newest")
  const [authorOnly, setAuthorOnly] = useState(false)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(10)

  const { data, isLoading, mutate } = useSWR(
    ["comments", articleId, sort, authorOnly, page, pageSize],
    () =>
      getComments({
        articleId,
        sort,
        authorOnlyId: authorOnly ? authorId : undefined,
        page,
        pageSize,
      }),
  )

  if (isLoading && !data) return <LoadingState />

  const totalPages = data ? Math.ceil(data.total / pageSize) : 0

  return (
    <div className="mt-6">
      <div className="mb-5 flex min-w-0 flex-wrap items-center justify-between gap-3 border-y border-border py-3">
        <div className="flex items-center gap-2 text-sm text-muted-foreground">
          <MessageSquare className="size-4 text-sakura-deep" />
          <span>{data?.total || 0} 条评论</span>
        </div>

        <div className="flex w-full min-w-0 flex-wrap items-center gap-2 sm:w-auto sm:justify-end">
          <select
            value={pageSize}
            onChange={(e) => {
              setPageSize(Number(e.target.value))
              setPage(1)
            }}
            className="rounded-sm border border-border bg-paper px-2 py-1 text-sm outline-none"
          >
            <option value={10}>10条/页</option>
            <option value={15}>15条/页</option>
            <option value={20}>20条/页</option>
          </select>

          <Button
            variant={authorOnly ? "default" : "outline"}
            size="sm"
            onClick={() => setAuthorOnly(!authorOnly)}
          >
            <Filter className="size-3.5" />
            只看楼主
          </Button>

          <div className="flex rounded-lg border border-border">
            <Button
              variant={sort === "newest" ? "default" : "ghost"}
              size="sm"
              onClick={() => setSort("newest")}
              className={cn(!(sort === "newest") && "rounded-r-none")}
            >
              <Clock className="size-3.5" />
              最新
            </Button>
            <Button
              variant={sort === "oldest" ? "default" : "ghost"}
              size="sm"
              onClick={() => setSort("oldest")}
              className={cn(!(sort === "oldest") && "rounded-l-none border-l")}
            >
              <Clock className="size-3.5" />
              最早
            </Button>
          </div>
        </div>
      </div>

      <div className="mb-6">
        <CommentForm articleId={articleId} onSubmit={() => { mutate(); setPage(1); }} />
      </div>

      {data && data.items.length > 0 ? (
        <div>
          {data.items.map((comment) => (
            <CommentItem
              key={comment.id}
              comment={comment}
              articleId={articleId}
              articleAuthorId={authorId}
              onChanged={() => void mutate()}
            />
          ))}
        </div>
      ) : (
        <EmptyState label="暂无评论" />
      )}

      {data && totalPages > 1 && (
        <div className="mt-6 flex items-center justify-center gap-2">
          <Button
            variant="outline"
            size="sm"
            disabled={page === 1}
            onClick={() => setPage(page - 1)}
          >
            <ChevronLeft className="size-4" />
          </Button>
          <span className="text-sm text-muted-foreground">
            第 {page} / {totalPages} 页
          </span>
          <Button
            variant="outline"
            size="sm"
            disabled={page === totalPages}
            onClick={() => setPage(page + 1)}
          >
            <ChevronRight className="size-4" />
          </Button>
        </div>
      )}
    </div>
  )
}
