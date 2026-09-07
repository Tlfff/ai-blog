"use client"

import { useState } from "react"
import Link from "next/link"
import { ChevronDown, ChevronUp, Flame } from "lucide-react"
import type { HotArticle } from "@/api/articles"
import { formatNumber } from "@/lib/format"
import { cn } from "@/lib/utils"

const RANK_TONES = [
  "bg-[var(--home-hot-coral)] text-white",
  "bg-[var(--home-hot-yellow)] text-white",
  "bg-[var(--home-hot-teal)] text-white",
  "bg-[var(--home-hot-lavender)] text-white",
  "bg-[var(--home-hot-muted)] text-white",
] as const

export function RankingList({ articles, loading = false, failed = false }: { articles: HotArticle[]; loading?: boolean; failed?: boolean }) {
  const [expanded, setExpanded] = useState(false)

  return (
    <section className="home-ranking mt-8 border-t border-[var(--home-divider)] pt-7 text-left lg:mt-9">
      <div className="flex items-center gap-2">
        <span className="grid size-7 place-items-center rounded-full bg-[var(--home-hot-coral-soft)] text-[var(--home-hot-coral)]"><Flame className="size-4 fill-current" /></span>
        <h3 className="font-playful text-xl font-bold text-[var(--home-text)]">热门文章</h3>
        <span className="ml-auto font-mono text-[0.55rem] uppercase tracking-[0.17em] text-[var(--home-faint)]">hot / top 5</span>
      </div>

      {loading ? <RankingLoading /> : failed ? <p className="py-7 text-center text-xs text-[var(--home-faint)]">热榜暂时不可用</p> : articles.length === 0 ? <p className="py-7 text-center text-xs text-[var(--home-faint)]">热榜还没有内容</p> : (
        <>
          <ol className="mt-4">
            {articles.map((article, index) => (
              <li key={article.id} className={cn(index >= 3 && !expanded && "hidden lg:block")}>
                <Link href={`/articles/${article.id}`} className="group grid grid-cols-[2rem_minmax(0,1fr)_auto] items-center gap-2.5 border-b border-[var(--home-divider)] py-3 last:border-b-0">
                  <span className={cn("grid size-7 place-items-center rounded-full text-xs font-bold", RANK_TONES[Math.min(index, RANK_TONES.length - 1)])}>{index + 1}</span>
                  <span className="line-clamp-2 text-xs font-semibold leading-5 text-[var(--home-text)] transition-colors group-hover:text-[var(--home-accent)]">{article.title}</span>
                  <span className={cn("inline-flex items-center gap-1 whitespace-nowrap text-[0.66rem] font-bold", index === 0 ? "text-[var(--home-hot-coral)]" : index === 1 ? "text-[var(--home-hot-yellow-deep)]" : index === 2 ? "text-[var(--home-hot-teal-deep)]" : "text-[var(--home-faint)]")}>
                    {index === 0 ? <Flame className="size-3 fill-current" /> : null}
                    {formatNumber(article.hot)}
                  </span>
                </Link>
              </li>
            ))}
          </ol>

          {articles.length > 3 ? (
            <button type="button" onClick={() => setExpanded((value) => !value)} className="mx-auto mt-3 flex items-center gap-2 rounded-full bg-[var(--home-hot-button)] px-5 py-2 text-[0.68rem] font-semibold text-[var(--home-muted)] transition-colors hover:text-[var(--home-accent)] lg:hidden" aria-expanded={expanded}>
              {expanded ? "收起热榜" : `查看完整热榜（${articles.length}）`}
              {expanded ? <ChevronUp className="size-3.5" /> : <ChevronDown className="size-3.5" />}
            </button>
          ) : null}
        </>
      )}
    </section>
  )
}

function RankingLoading() {
  return <div className="mt-4 space-y-3">{Array.from({ length: 3 }, (_, index) => <div key={index} className="flex items-center gap-3"><span className="size-7 animate-pulse rounded-full bg-[var(--home-divider)]" /><span className="h-3 flex-1 animate-pulse rounded-full bg-[var(--home-divider)]" /><span className="h-3 w-8 animate-pulse rounded-full bg-[var(--home-divider)]" /></div>)}</div>
}
