"use client"

import { SITE_IMAGES } from "@/lib/site-images"
import { Suspense, useEffect, useState, type FormEvent } from "react"
import Image from "next/image"
import Link from "next/link"
import { useRouter, useSearchParams } from "next/navigation"
import { AlertCircle, ArrowRight, BookOpen, ChevronLeft, ChevronRight, RefreshCw, Search, Sparkles } from "lucide-react"
import useSWR from "swr"
import { searchArticles } from "@/api/articles"
import { SearchResultCard } from "@/components/search/search-result-card"
import { Container } from "@/components/layout/container"
import { SiteShell } from "@/components/layout/site-shell"
import { Button } from "@/components/ui/button"
import { LoadingState } from "@/components/ui/spinner"

const PAGE_SIZE = 10

function parsePage(value: string | null) {
  const page = Number.parseInt(value ?? "1", 10)
  return Number.isFinite(page) && page >= 1 ? page : 1
}

function SearchPageContent() {
  const router = useRouter()
  const searchParams = useSearchParams()
  const queryKeyword = searchParams.get("q")?.trim() ?? ""
  const page = parsePage(searchParams.get("page"))
  const [keyword, setKeyword] = useState(queryKeyword)

  useEffect(() => setKeyword(queryKeyword), [queryKeyword])

  const { data, error, isLoading, isValidating, mutate } = useSWR(
    ["article-search", queryKeyword || "all", page, PAGE_SIZE],
    () => searchArticles({ keyword: queryKeyword, page, pageSize: PAGE_SIZE }),
  )
  const totalPages = data ? Math.max(1, Math.ceil(data.total / data.pageSize)) : 1
  const startIndex = data ? (data.page - 1) * data.pageSize : 0

  function navigateToSearch(nextKeyword: string, nextPage = 1) {
    const params = new URLSearchParams({ page: String(nextPage) })
    if (nextKeyword) params.set("q", nextKeyword)
    router.push(`/search?${params.toString()}`)
  }

  function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const nextKeyword = keyword.trim()
    navigateToSearch(nextKeyword)
  }

  function handlePageChange(nextPage: number) {
    if (nextPage < 1 || nextPage > totalPages) return
    navigateToSearch(queryKeyword, nextPage)
    window.scrollTo({ top: 0, behavior: "smooth" })
  }

  return (
    <SiteShell immersiveHeader>
      <div className="search-page min-h-screen overflow-hidden bg-[var(--search-background)] text-[var(--search-ink)]">
        <section className="relative isolate min-h-[330px] overflow-hidden text-white sm:min-h-[390px]">
          <Image src={SITE_IMAGES.pages.primarySky} alt="" fill priority sizes="100vw" className="object-cover object-[center_28%]" />
          <div className="absolute inset-0 bg-[linear-gradient(90deg,rgba(9,61,96,0.86),rgba(25,130,165,0.57),rgba(48,166,157,0.34))]" aria-hidden />
          <div className="absolute inset-x-0 bottom-0 h-40 bg-gradient-to-t from-[#19546c]/55 to-transparent" aria-hidden />

          <Container className="relative flex min-h-[330px] flex-col justify-center pb-16 pt-24 sm:min-h-[390px] sm:pb-24">
            <div className="flex flex-col gap-5 sm:flex-row sm:items-start sm:justify-between">
              <div className="max-w-4xl">
                <p className="font-mono text-[0.66rem] font-semibold uppercase tracking-[0.27em] text-white/78">search / archive</p>
                <h1 className="mt-4 font-playful text-[clamp(2.25rem,3.1vw,3rem)] font-bold leading-[1.08] tracking-[-0.045em] [text-shadow:0_3px_18px_rgba(0,28,48,0.28)] lg:whitespace-nowrap">从一个关键词，找到值得读的文章。</h1>
                <p className="mt-4 text-sm leading-7 text-white/86 sm:text-base">搜索标题、正文、标签和中文标题的完整拼音。</p>
              </div>
              <div className="hidden min-w-44 rounded-2xl border border-white/24 bg-[#173b5c]/25 px-5 py-4 backdrop-blur-sm sm:block">
                <p className="font-mono text-[0.58rem] uppercase tracking-[0.18em] text-white/68">result mode</p>
                <p className="mt-1 font-playful text-2xl font-bold">文章列表</p>
                <p className="mt-1 text-xs text-white/72">按相关性排列</p>
              </div>
            </div>

            <form onSubmit={handleSubmit} role="search" className="relative mt-6 max-w-4xl sm:mt-7">
              <Search className="pointer-events-none absolute left-4 top-1/2 size-5 -translate-y-1/2 text-[#5f7d8c]" aria-hidden />
              <input
                type="search"
                value={keyword}
                onChange={(event) => setKeyword(event.target.value)}
                placeholder="搜索标题、正文、标签或完整拼音..."
                aria-label="文章搜索关键词"
                className="h-14 w-full rounded-full border border-white/65 bg-white/96 pl-12 pr-28 text-sm text-[#18354b] shadow-[0_12px_35px_rgba(15,70,91,0.18)] outline-none transition placeholder:text-[#92a7af] focus:ring-4 focus:ring-white/25 sm:pr-32"
              />
              <Button type="submit" disabled={isValidating} className="absolute right-1.5 top-1/2 h-11 -translate-y-1/2 rounded-full bg-[#245f80] px-6 text-white hover:bg-[#1d526f]">
                {isValidating ? "搜索中" : "搜索"}
              </Button>
            </form>
          </Container>

          <div className="pointer-events-none absolute inset-x-0 bottom-0 z-10 h-20 sm:h-28" aria-hidden>
            <svg viewBox="0 0 1440 112" preserveAspectRatio="none" className="h-full w-full">
              <path d="M0 48C210 82 398 77 590 61C815 42 994 27 1168 51C1285 67 1366 70 1440 60V112H0Z" fill="var(--search-wave)" />
              <path d="M0 72C188 58 353 99 588 87C818 75 1002 47 1190 70C1290 82 1365 88 1440 80V112H0Z" fill="var(--search-background)" />
            </svg>
          </div>
        </section>

        <Container className="relative z-20 -mt-9 max-w-[1200px] pb-16 sm:-mt-14 sm:pb-24">
          <div className="grid gap-5 lg:grid-cols-[minmax(0,1fr)_306px]">
            <section className="search-results-panel overflow-hidden rounded-[1.25rem] border border-[var(--search-border)] bg-[var(--search-card)] shadow-[0_16px_46px_var(--search-shadow)]" aria-live="polite">
              <SearchResultsHeader queryKeyword={queryKeyword} data={data} totalPages={totalPages} />
              <SearchResultsBody queryKeyword={queryKeyword} data={data} error={error} isLoading={isLoading} startIndex={startIndex} mutate={mutate} />
              {data && data.items.length > 0 && totalPages > 1 ? (
                <nav className="flex items-center justify-center gap-3 border-t border-[var(--search-border)] px-5 py-4" aria-label="搜索结果分页">
                  <Button variant="outline" size="sm" disabled={data.page <= 1 || isValidating} onClick={() => handlePageChange(data.page - 1)} className="rounded-full border-[var(--search-border-strong)] bg-transparent"><ChevronLeft className="size-4" />上一页</Button>
                  <span className="min-w-20 text-center text-xs text-[var(--search-muted)]">第 {data.page} / {totalPages} 页</span>
                  <Button variant="outline" size="sm" disabled={data.page >= totalPages || isValidating} onClick={() => handlePageChange(data.page + 1)} className="rounded-full border-[var(--search-border-strong)] bg-transparent">下一页<ChevronRight className="size-4" /></Button>
                </nav>
              ) : null}
            </section>

            <aside className="space-y-5">
              <section className="rounded-[1.25rem] border border-[var(--search-border)] bg-[var(--search-card)] p-5 shadow-[0_16px_46px_var(--search-shadow)]">
                <div className="rounded-2xl bg-[linear-gradient(135deg,#398fc2,#69c8c2)] p-5 text-white"><p className="font-mono text-[0.58rem] font-semibold uppercase tracking-[0.16em] text-white/78">search notes</p><p className="mt-2 font-playful text-xl font-bold">文章列表结果</p></div>
                <h2 className="search-heading mt-5 font-playful text-xl font-bold">搜索说明</h2>
                <p className="mt-3 text-sm leading-7 text-[var(--search-muted)]">标题和摘要会保留命中高亮，标签可以继续作为下一次搜索入口。</p>
                <p className="mt-4 inline-flex items-center gap-2 rounded-full bg-[var(--search-teal-soft)] px-3 py-1.5 text-xs font-medium text-[var(--search-teal-deep)]"><span className="size-2 rounded-full bg-[var(--search-teal)]" />支持正文与标签检索</p>
              </section>

              <section className="rounded-[1.25rem] border border-[var(--search-border)] bg-[var(--search-card)] p-5 shadow-[0_16px_46px_var(--search-shadow)]">
                <h2 className="search-heading font-playful text-xl font-bold">没有找到？</h2>
                <p className="mt-3 text-sm leading-7 text-[var(--search-muted)]">可以尝试更短的关键词，或使用中文标题的完整拼音。</p>
                <Link href="/#latest" className="mt-5 flex items-center rounded-xl border-t border-[var(--search-border)] pt-4 text-sm font-semibold text-[var(--search-sky-deep)]">返回首页继续浏览<ArrowRight className="ml-auto size-4" /></Link>
              </section>
            </aside>
          </div>
        </Container>
      </div>
    </SiteShell>
  )
}

function SearchResultsHeader({ queryKeyword, data, totalPages }: { queryKeyword: string; data: Awaited<ReturnType<typeof searchArticles>> | undefined; totalPages: number }) {
  return (
    <div className="flex items-start justify-between gap-4 border-b border-[var(--search-border)] px-5 py-5 sm:px-7">
      <div>
        <p className="font-mono text-[0.62rem] font-semibold uppercase tracking-[0.2em] text-[var(--search-sky-deep)]">result / relevance</p>
        <p className="mt-2 text-sm text-[var(--search-muted)]">
          {queryKeyword ? <>“<strong className="text-[var(--search-ink)]">{queryKeyword}</strong>” {data ? <>共找到 <strong className="text-[var(--search-ink)]">{data.total}</strong> 篇文章</> : "正在查找文章"}</> : data ? <>当前共 <strong className="text-[var(--search-ink)]">{data.total}</strong> 篇文章</> : "正在加载全部文章"}
        </p>
      </div>
      {data ? <span className="hidden font-mono text-[0.58rem] uppercase tracking-[0.15em] text-[var(--search-faint)] sm:block">page {String(data.page).padStart(2, "0")} / {String(totalPages).padStart(2, "0")}</span> : null}
    </div>
  )
}

function SearchResultsBody({ queryKeyword, data, error, isLoading, startIndex, mutate }: { queryKeyword: string; data: Awaited<ReturnType<typeof searchArticles>> | undefined; error: unknown; isLoading: boolean; startIndex: number; mutate: () => Promise<unknown> }) {
  if (isLoading && !data) return <LoadingState label="正在搜索文章..." />
  if (error) return <SearchState icon={<AlertCircle className="size-7" />} title="搜索暂时不可用" description={error instanceof Error ? error.message : "文章搜索服务暂不可用"} action={<Button variant="outline" onClick={() => void mutate()} className="mt-5 rounded-full border-[var(--search-border-strong)] bg-transparent"><RefreshCw className="size-4" />重新搜索</Button>} tone="coral" />
  if (!data || data.items.length === 0) return <SearchState icon={<Sparkles className="size-7" />} title={queryKeyword ? "没有找到相关文章" : "暂时没有文章"} description={queryKeyword ? `没有找到与“${queryKeyword}”相关的内容，请尝试缩短关键词或使用完整拼音。` : "文章发布后会显示在这里。"} />
  return <div className="space-y-3 p-4 sm:p-5">{data.items.map((article, index) => <SearchResultCard key={article.id} article={article} index={startIndex + index} keyword={queryKeyword} />)}</div>
}

function SearchState({ icon, title, description, action, tone = "sky" }: { icon: React.ReactNode; title: string; description: string; action?: React.ReactNode; tone?: "sky" | "coral" }) {
  return <div className="grid min-h-80 place-items-center px-6 py-12 text-center"><div><span className={`mx-auto grid size-14 place-items-center rounded-2xl ${tone === "coral" ? "bg-[var(--search-coral-soft)] text-[var(--search-coral)]" : "bg-[var(--search-sky-soft)] text-[var(--search-sky-deep)]"}`}>{icon}</span><h2 className="search-heading mt-5 font-playful text-2xl font-bold">{title}</h2><p className="mx-auto mt-2 max-w-md text-sm leading-7 text-[var(--search-muted)]">{description}</p>{action}</div></div>
}

export default function SearchPage() {
  return <Suspense fallback={<LoadingState label="正在加载搜索页面..." />}><SearchPageContent /></Suspense>
}
