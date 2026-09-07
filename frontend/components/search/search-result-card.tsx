import Link from "next/link"
import { ArrowRight } from "lucide-react"
import { Badge } from "@/components/ui/badge"
import type { ArticleSearchItem } from "@/api/articles"
import { SearchHighlight } from "./search-highlight"

interface SearchResultCardProps {
  article: ArticleSearchItem
  index: number
  keyword: string
}

const ACCENTS = [
  "var(--search-sky)",
  "var(--search-teal)",
  "var(--search-yellow)",
  "var(--search-lavender)",
] as const

function hasExactHighlight(value: string, keyword: string) {
  const normalizedKeyword = keyword.trim().toLocaleLowerCase()
  if (!normalizedKeyword) return false

  return Array.from(value.matchAll(/<em>([\s\S]*?)<\/em>/gi)).some(
    (match) => match[1].trim().toLocaleLowerCase() === normalizedKeyword,
  )
}

export function SearchResultCard({ article, index, keyword }: SearchResultCardProps) {
  const title = article.titleHighlight || article.title
  const articleNumber = String(index + 1).padStart(2, "0")
  const accent = ACCENTS[index % ACCENTS.length]
  const normalizedKeyword = keyword.trim().toLocaleLowerCase()
  const matchedTags = new Set(
    article.tags
      .filter((tag) => normalizedKeyword && tag.toLocaleLowerCase() === normalizedKeyword)
      .map((tag) => tag.toLocaleLowerCase()),
  )
  const matchedAreas = [
    hasExactHighlight(title, keyword) ? "标题" : "",
    hasExactHighlight(article.summary, keyword) ? "正文" : "",
    matchedTags.size > 0 ? "标签" : "",
  ].filter(Boolean)

  return (
    <article
      className="search-result-card group relative overflow-hidden rounded-2xl border border-[var(--search-border)] bg-[var(--search-result-card)] shadow-[0_7px_20px_var(--search-shadow-soft)]"
      style={{ "--result-accent": accent, animationDelay: `${Math.min(index, 6) * 70}ms` } as React.CSSProperties}
    >
      <span aria-hidden className="absolute inset-y-0 left-0 w-1 bg-[var(--result-accent)]" />
      <div className="p-4 pl-6 sm:p-5 sm:pl-7">
        <div className="flex items-start gap-4">
          <div className="hidden w-8 shrink-0 border-r border-[var(--search-border)] pr-3 sm:block">
            <span className="font-mono text-[0.62rem] font-semibold tracking-[0.12em] text-[var(--result-accent)]">{articleNumber}</span>
            <span className="mt-3 block h-9 w-px bg-[var(--search-border)]" aria-hidden />
          </div>
          <div className="min-w-0 flex-1">
            <div className="flex items-start justify-between gap-3">
              <h2 className="search-heading min-w-0 font-playful text-lg font-bold leading-7 sm:text-xl">
                <Link href={`/articles/${article.id}`} className="line-clamp-2 hover:text-[var(--search-sky-deep)]"><SearchHighlight value={title} /></Link>
              </h2>
              <Link href={`/articles/${article.id}`} aria-label={`阅读文章：${article.title}`} className="mt-0.5 grid size-8 shrink-0 place-items-center rounded-full text-[var(--search-faint)] transition-transform group-hover:translate-x-1 group-hover:bg-[var(--search-tag)] group-hover:text-[var(--result-accent)]"><ArrowRight className="size-4" /></Link>
            </div>

            {article.summary ? <p className="mt-2 line-clamp-2 text-sm leading-6 text-[var(--search-muted)]"><SearchHighlight value={article.summary} /></p> : <p className="mt-2 text-xs text-[var(--search-faint)]">关键词命中标题或标签</p>}

            <div className="mt-4 flex flex-wrap items-center gap-2">
              {article.tags.map((tag) => {
                const matched = matchedTags.has(tag.toLocaleLowerCase())
                return <Link key={tag} href={`/search?q=${encodeURIComponent(tag)}&page=1`}><Badge className="rounded-full border-0 bg-[var(--search-tag)] px-2.5 py-1 text-xs font-medium text-[var(--search-muted)] transition-colors hover:bg-[var(--search-sky-soft)] hover:text-[var(--search-sky-deep)]"><SearchHighlight value={matched ? `<em>${tag}</em>` : tag} /></Badge></Link>
              })}
              {matchedAreas.length > 0 ? <span className="ml-auto hidden text-xs text-[var(--search-faint)] sm:inline">命中{matchedAreas.join("、")}「{keyword}」</span> : null}
            </div>
          </div>
        </div>
      </div>
    </article>
  )
}
