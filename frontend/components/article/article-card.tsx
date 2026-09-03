import Link from "next/link"
import { CalendarDays, Eye, Heart, MessageSquare, PenLine } from "lucide-react"
import type { Article } from "@/types"
import { formatDate, formatNumber } from "@/lib/format"
import { cn } from "@/lib/utils"

interface ArticleCardProps {
  article: Article
  index?: number
}

const FALLBACK_COVERS = [
  "/go-programming-blog-cover.png",
  "/frontend-engineering-workflow.png",
  "/database-optimization-concept.png",
  "/distributed-systems-network.png",
  "/react-server-components.png",
  "/career-growth-path.png",
] as const

export function ArticleCard({ article, index = 0 }: ArticleCardProps) {
  const imageOnLeft = index % 2 === 0
  const cover = article.cover || FALLBACK_COVERS[index % FALLBACK_COVERS.length]
  const category = article.tags[0]?.name || (index % 2 === 0 ? "技术随笔" : "开发记录")

  return (
    <article
      className="post-item-card group relative flex min-h-[14.5rem] border-b border-[var(--home-divider)] text-[var(--home-text)]"
    >
      <Link
        href={`/articles/${article.id}`}
        aria-label={`阅读：${article.title}`}
        className={cn(
          "post-cover relative block h-[14.5rem] w-[45%] shrink-0 overflow-hidden bg-[var(--home-image-surface)]",
          imageOnLeft ? "post-cover-left order-1" : "post-cover-right order-2",
        )}
      >
        {/* eslint-disable-next-line @next/next/no-img-element */}
        <img
          src={cover}
          alt={article.title}
          className="h-full w-full cursor-pointer object-cover transition duration-500 ease-out group-hover:scale-110 group-hover:rotate-[1.5deg]"
        />
      </Link>

      <div
        className={cn(
          "post-content flex min-w-0 w-[55%] flex-col gap-2 px-4 pb-4 pt-4 sm:px-5",
          imageOnLeft ? "order-2" : "order-1",
        )}
      >
        <div className="flex w-full items-start justify-between gap-3 text-xs text-[var(--home-faint)]">
          <Link
            href={`/search?q=${encodeURIComponent(category)}&page=1`}
            className="inline-flex min-w-0 items-center gap-1 truncate transition-colors hover:text-[var(--home-accent)]"
          >
            <PenLine className="size-3 shrink-0" />
            <span className="truncate">{category}</span>
          </Link>
          <div className="flex shrink-0 items-center gap-3">
            {article.createdAt && (
              <time className="inline-flex items-center gap-1">
                <CalendarDays className="size-3" />
                {formatDate(article.createdAt)}
              </time>
            )}
            <span className="hidden items-center gap-1 lg:inline-flex">
              <PenLine className="size-3" />
              {article.content.length || "—"} 字
            </span>
          </div>
        </div>

        <Link href={`/articles/${article.id}`} className="mt-1 block min-w-0">
          <h3 className="line-clamp-1 truncate font-playful text-lg font-bold leading-snug text-[var(--home-accent)] transition-colors duration-300 hover:text-[var(--home-accent-hover)] sm:text-xl">
            {article.title}
          </h3>
        </Link>

        <p className="line-clamp-3 min-h-[4.5rem] text-sm leading-6 text-[var(--home-muted)]">
          {article.summary || "继续阅读这篇文章，看看这里记录了哪些技术思考与生活片段。"}
        </p>

        <div className="mt-auto flex items-center justify-end gap-3 pt-1">
          <div className="flex shrink-0 items-center gap-3 text-xs text-[var(--home-faint)]">
            <span className="inline-flex items-center gap-1">
              <Eye className="size-3.5" />
              {formatNumber(article.views)}
            </span>
            <span className="hidden items-center gap-1 sm:inline-flex">
              <Heart className="size-3.5" />
              {formatNumber(article.likes)}
            </span>
            <Link
              href={`/articles/${article.id}#comments`}
              className="inline-flex items-center gap-1 transition-colors hover:text-[var(--home-accent)]"
            >
              <MessageSquare className="size-3.5" />
              {formatNumber(article.commentsCount)}
            </Link>
          </div>
        </div>
      </div>
    </article>
  )
}
