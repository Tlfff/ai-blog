"use client"

import { Suspense, useEffect, useState, type FormEvent } from "react"
import Link from "next/link"
import { useRouter, useSearchParams } from "next/navigation"
import useSWR from "swr"
import {
  AlertCircle,
  ArrowLeft,
  ChevronLeft,
  ChevronRight,
  RefreshCw,
  Search,
} from "lucide-react"
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

  useEffect(() => {
    setKeyword(queryKeyword)
  }, [queryKeyword])

  const { data, error, isLoading, isValidating, mutate } = useSWR(
    queryKeyword ? ["article-search", queryKeyword, page, PAGE_SIZE] : null,
    () => searchArticles({ keyword: queryKeyword, page, pageSize: PAGE_SIZE }),
  )

  const totalPages = data ? Math.max(1, Math.ceil(data.total / data.pageSize)) : 1

  function navigateToSearch(nextKeyword: string, nextPage = 1) {
    const params = new URLSearchParams({
      q: nextKeyword,
      page: String(nextPage),
    })
    router.push(`/search?${params.toString()}`)
  }

  function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const nextKeyword = keyword.trim()
    if (!nextKeyword) return
    navigateToSearch(nextKeyword)
  }

  function handlePageChange(nextPage: number) {
    if (!queryKeyword || nextPage < 1 || nextPage > totalPages) return
    navigateToSearch(queryKeyword, nextPage)
    window.scrollTo({ top: 0, behavior: "smooth" })
  }

  const startIndex = data ? (data.page - 1) * data.pageSize : 0

  return (
    <SiteShell>
      <Container className="py-8 md:py-12">
        <div className="border-b border-border pb-7">
          <Link
            href="/"
            className="label-meta inline-flex items-center gap-2 text-ink transition-colors hover:text-sakura-deep"
          >
            <ArrowLeft className="size-4" />
            back / home
          </Link>

          <div className="mt-6 grid gap-6 lg:grid-cols-[0.72fr_1.28fr] lg:items-end">
            <div>
              <p className="label-meta text-sakura-deep">search / archive</p>
              <h1 className="title-display mt-3 text-4xl text-ink md:text-5xl">文章搜索</h1>
              <p className="mt-3 max-w-md text-sm leading-7 text-ink-soft">
                支持搜索标题、正文、标签和标题完整拼音，结果按相关性排序。
              </p>
            </div>

            <form onSubmit={handleSubmit} className="relative" role="search">
              <Search className="pointer-events-none absolute left-4 top-1/2 size-5 -translate-y-1/2 text-ink-soft" aria-hidden />
              <input
                type="search"
                value={keyword}
                onChange={(event) => setKeyword(event.target.value)}
                placeholder="搜索标题、正文、标签或完整拼音..."
                aria-label="文章搜索关键词"
                className="h-13 w-full rounded-sm border border-ink bg-paper pl-12 pr-24 text-sm text-ink outline-none transition-shadow placeholder:text-ink-soft/65 focus:border-sakura-deep focus:ring-2 focus:ring-sakura/30"
              />
              <Button
                type="submit"
                disabled={!keyword.trim() || isValidating}
                className="absolute right-1.5 top-1/2 -translate-y-1/2 rounded-sm"
              >
                {isValidating ? "搜索中" : "搜索"}
              </Button>
            </form>
          </div>
        </div>

        <section className="pt-7" aria-live="polite">
          {!queryKeyword ? (
            <div className="border border-dashed border-border bg-paper-deep px-6 py-16 text-center">
              <Search className="mx-auto size-8 text-sakura-deep" aria-hidden />
              <h2 className="title-display mt-4 text-2xl text-ink">从一个关键词开始</h2>
              <p className="mt-2 text-sm leading-7 text-ink-soft">
                试试技术名称、文章主题、标签，或中文标题的完整拼音。
              </p>
            </div>
          ) : isLoading && !data ? (
            <LoadingState label="正在搜索文章..." />
          ) : error ? (
            <div className="border border-destructive/35 bg-destructive/5 px-6 py-12 text-center">
              <AlertCircle className="mx-auto size-8 text-destructive" aria-hidden />
              <h2 className="title-display mt-4 text-2xl text-ink">搜索暂时不可用</h2>
              <p className="mt-2 text-sm text-ink-soft">
                {error instanceof Error ? error.message : "文章搜索服务暂不可用"}
              </p>
              <Button variant="outline" className="mt-5" onClick={() => mutate()}>
                <RefreshCw className="size-4" />
                重新搜索
              </Button>
            </div>
          ) : !data || data.items.length === 0 ? (
            <div className="border border-dashed border-border bg-paper-deep px-6 py-16 text-center">
              <Search className="mx-auto size-8 text-ink-soft" aria-hidden />
              <h2 className="title-display mt-4 text-2xl text-ink">没有找到相关文章</h2>
              <p className="mt-2 text-sm leading-7 text-ink-soft">
                没有找到与“{queryKeyword}”相关的内容，请尝试缩短关键词或使用完整拼音。
              </p>
            </div>
          ) : (
            <>
              <div className="mb-5 flex flex-wrap items-end justify-between gap-3">
                <div>
                  <p className="label-meta text-sakura-deep">result / relevance</p>
                  <p className="mt-2 text-sm text-ink-soft">
                    “<span className="font-medium text-ink">{queryKeyword}</span>”共找到{" "}
                    <span className="font-medium text-ink">{data.total}</span> 篇文章
                  </p>
                </div>
                <span className="label-meta">
                  page {data.page} / {totalPages}
                </span>
              </div>

              <div className="space-y-4">
                {data.items.map((article, index) => (
                  <SearchResultCard
                    key={article.id}
                    article={article}
                    index={startIndex + index}
                    keyword={queryKeyword}
                  />
                ))}
              </div>

              {totalPages > 1 && (
                <nav className="mt-8 flex items-center justify-center gap-3" aria-label="搜索结果分页">
                  <Button
                    variant="outline"
                    size="sm"
                    disabled={data.page <= 1 || isValidating}
                    onClick={() => handlePageChange(data.page - 1)}
                  >
                    <ChevronLeft className="size-4" />
                    上一页
                  </Button>
                  <span className="min-w-24 text-center text-sm text-ink-soft">
                    第 {data.page} / {totalPages} 页
                  </span>
                  <Button
                    variant="outline"
                    size="sm"
                    disabled={data.page >= totalPages || isValidating}
                    onClick={() => handlePageChange(data.page + 1)}
                  >
                    下一页
                    <ChevronRight className="size-4" />
                  </Button>
                </nav>
              )}
            </>
          )}
        </section>
      </Container>
    </SiteShell>
  )
}

export default function SearchPage() {
  return (
    <Suspense fallback={<LoadingState label="正在加载搜索页面..." />}>
      <SearchPageContent />
    </Suspense>
  )
}
