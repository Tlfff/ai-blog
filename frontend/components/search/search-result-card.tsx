import Link from "next/link"
import { ArrowUpRight } from "lucide-react"
import { Badge } from "@/components/ui/badge"
import type { ArticleSearchItem } from "@/api/articles"
import { SearchHighlight } from "./search-highlight"

interface SearchResultCardProps {
  article: ArticleSearchItem
  index: number
  keyword: string
}

const ACCENTS = [
  "var(--kessoku-bocchi)",
  "var(--kessoku-nijika)",
  "var(--kessoku-ryo)",
  "var(--kessoku-kita)",
]

export function SearchResultCard({ article, index, keyword }: SearchResultCardProps) {
  const title = article.titleHighlight || article.title
  const articleNumber = String(index + 1).padStart(2, "0")
  const accent = ACCENTS[index % ACCENTS.length]

  return (
    <article className="card-paper group relative overflow-hidden rounded-sm opacity-0 animate-cut-in-up" style={{ animationDelay: `${Math.min(index, 6) * 70}ms` }}>
      <span aria-hidden className="absolute left-0 top-0 h-full w-1" style={{ backgroundColor: accent }} />

      <div className="flex gap-4 p-5 sm:gap-6 sm:p-6">
        <div className="hidden w-10 shrink-0 border-r border-border pr-4 sm:block">
          <span className="label-meta text-sakura-deep">{articleNumber}</span>
          <span className="mt-2 block h-10 w-px bg-border" aria-hidden />
          <span className="label-meta [writing-mode:vertical-rl] text-ink-soft">result</span>
        </div>

        <div className="min-w-0 flex-1">
          <Link href={`/articles/${article.id}`} className="block">
            <div className="flex items-start justify-between gap-4">
              <h2 className="title-display text-xl leading-snug text-ink sm:text-2xl">
                <span className="underline-sweep line-clamp-2">
                  <SearchHighlight value={title} />
                </span>
              </h2>
              <ArrowUpRight className="mt-1 size-5 shrink-0 text-ink-soft transition-transform group-hover:-translate-y-0.5 group-hover:translate-x-0.5 group-hover:text-sakura-deep" aria-hidden />
            </div>
          </Link>

          {article.summary ? (
            <p className="mt-3 line-clamp-3 text-sm leading-7 text-ink-soft">
              <SearchHighlight value={article.summary} />
            </p>
          ) : (
            <p className="mt-3 text-xs text-ink-soft/75">关键词命中标题或标签</p>
          )}

          <div className="mt-4 flex flex-wrap items-center gap-2">
            {article.tags.map((tag) => (
              <Link key={tag} href={`/search?q=${encodeURIComponent(tag)}&page=1`}>
                <Badge className="rounded-sm border border-border bg-transparent font-normal text-ink-soft transition-colors hover:border-sakura-deep hover:text-sakura-deep">
                  {tag}
                </Badge>
              </Link>
            ))}
            <span className="ml-auto hidden text-xs text-ink-soft/70 sm:inline">
              命中「{keyword}」
            </span>
          </div>
        </div>
      </div>
    </article>
  )
}
