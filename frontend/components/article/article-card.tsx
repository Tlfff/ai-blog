import Link from "next/link"
import { Eye, Heart, MessageSquare } from "lucide-react"
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
  const reversed = index % 2 === 1
  const cover = article.cover || FALLBACK_COVERS[index % FALLBACK_COVERS.length]
  const category = article.tags[0]?.name || (index % 2 === 0 ? "技术随笔" : "开发记录")

  return (
    <article
      className="home-article-row group border-b border-white/12 py-7 opacity-0 animate-cut-in-up first:pt-0 sm:py-9"
      style={{ animationDelay: `${Math.min(index, 6) * 70}ms` }}
    >
      <div className="grid items-center gap-5 sm:gap-7 md:grid-cols-[minmax(0,3.8fr)_minmax(0,6.2fr)] lg:gap-9">
        <Link
          href={`/articles/${article.id}`}
          className={cn(
            "relative aspect-[16/9] min-w-0 overflow-hidden bg-white/5 md:row-start-1",
            reversed
              ? "md:col-start-2 [clip-path:polygon(10%_0,100%_0,100%_100%,0_100%)]"
              : "md:col-start-1 [clip-path:polygon(0_0,100%_0,90%_100%,0_100%)]",
          )}
        >
          {/* eslint-disable-next-line @next/next/no-img-element */}
          <img
            src={cover}
            alt={article.title}
            className="h-full w-full object-cover transition-transform duration-500 ease-out group-hover:scale-[1.045]"
          />
          <span
            className="absolute inset-0 bg-gradient-to-t from-black/25 via-transparent to-transparent opacity-60 transition-opacity group-hover:opacity-25"
            aria-hidden
          />
        </Link>

        <div
          className={cn(
            "min-w-0 md:row-start-1",
            reversed ? "md:col-start-1 md:pr-2" : "md:col-start-2 md:pl-1",
          )}
        >
          <p className="text-xs font-medium tracking-[0.12em] text-white/50">
            {category}
          </p>

          <Link href={`/articles/${article.id}`} className="mt-2.5 block">
            <h3 className="font-playful text-[1.35rem] font-bold leading-snug tracking-[-0.01em] text-[#f46f98] transition-colors group-hover:text-[#ff91b2] sm:text-2xl">
              {article.title}
            </h3>
            <p className="mt-3 line-clamp-2 text-sm leading-7 text-white/68 sm:text-[0.95rem]">
              {article.summary || "继续阅读这篇文章，看看这里记录了哪些技术思考与生活片段。"}
            </p>
          </Link>

          {article.tags.length > 0 ? (
            <div className="mt-4 flex flex-wrap gap-x-3 gap-y-1.5">
              {article.tags.slice(0, 3).map((tag) => (
                <Link
                  key={tag.id}
                  href={`/search?q=${encodeURIComponent(tag.name)}&page=1`}
                  className="text-xs text-[#f3a4ba]/80 transition-colors hover:text-[#f3a4ba]"
                >
                  #{tag.name}
                </Link>
              ))}
            </div>
          ) : null}

          <div className="mt-5 flex flex-wrap items-center gap-x-4 gap-y-2 text-xs text-white/48">
            {article.createdAt ? <time>{formatDate(article.createdAt)}</time> : null}
            <span className="inline-flex items-center gap-1.5">
              <Eye className="size-3.5" />
              {formatNumber(article.views)}
            </span>
            <span className="inline-flex items-center gap-1.5">
              <Heart className="size-3.5" />
              {formatNumber(article.likes)}
            </span>
            <Link
              href={`/articles/${article.id}#comments`}
              className="inline-flex items-center gap-1.5 transition-colors hover:text-[#f3a4ba]"
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
