import Link from "next/link"
import { Flame } from "lucide-react"
import { formatNumber } from "@/lib/format"
import { cn } from "@/lib/utils"
import type { Article } from "@/types"

export function RankingList({ articles }: { articles: Article[] }) {
  return (
    <div className="mt-4 p-4 card-paper rounded-md">
      <h3 className="mb-3 flex items-center gap-2 text-sm font-semibold">
        <div className="p-1 rounded-lg bg-primary/10">
          <Flame className="size-4 text-primary" />
        </div>
        热门文章排行
      </h3>
      <div className="space-y-2">
        {articles.map((article, i) => {
          const rank = i + 1
          const heat = article.likes * 3 + article.commentsCount * 5 + Math.round(article.views * 0.1)
          return (
            <Link
              key={article.id}
              href={`/articles/${article.id}`}
              className="group flex items-start gap-3 rounded-xl px-3 py-2.5 transition-all duration-200 hover:bg-primary/5"
            >
              <span
                className={cn(
                  "mt-0.5 flex size-6 shrink-0 items-center justify-center rounded-lg text-xs font-bold",
                  rank === 1
                    ? "bg-sakura-deep text-primary-foreground"
                    : rank <= 3
                    ? "bg-primary/20 text-primary"
                    : "bg-card/50 text-muted-foreground",
                )}
              >
                {rank}
              </span>
              <div className="min-w-0 flex-1">
                <p className="line-clamp-2 text-sm leading-snug transition-colors group-hover:text-primary">
                  {article.title}
                </p>
                <div className="mt-1.5 flex items-center gap-2 text-xs text-muted-foreground">
                  <span className="inline-flex items-center gap-1 text-primary/80">
                    <Flame className="size-3" />
                    {formatNumber(heat)}
                  </span>
                  <span>{formatNumber(article.views)}阅</span>
                  <span>{formatNumber(article.likes)}赞</span>
                  <span>{formatNumber(article.commentsCount)}评</span>
                </div>
              </div>
            </Link>
          )
        })}
      </div>
    </div>
  )
}