"use client"

import { use, useEffect, useState } from "react"
import Image from "next/image"
import Link from "next/link"
import useSWR from "swr"
import {
  ArrowLeft,
  CalendarDays,
  Clock3,
  Copy,
  Eye,
  Heart,
  Share2,
  ThumbsUp,
} from "lucide-react"
import { getArticleById, recordView, toggleArticleLike } from "@/api/articles"
import { ArticleContent } from "@/components/article/article-content"
import { ArticleToc, extractArticleHeadings } from "@/components/article/article-toc"
import { CommentList } from "@/components/comment/comment-list"
import { Container } from "@/components/layout/container"
import { SiteShell } from "@/components/layout/site-shell"
import { Avatar } from "@/components/ui/avatar"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { LoadingState } from "@/components/ui/spinner"
import { useAuth } from "@/hooks/use-auth"
import { formatDate, formatNumber } from "@/lib/format"
import { cn } from "@/lib/utils"
import { ReadingProgress } from "@/components/article/reading-progress"

export default function ArticleDetailPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params)
  const { isLoggedIn } = useAuth()
  const [likes, setLikes] = useState(0)
  const [liked, setLiked] = useState(false)
  const [shareCopied, setShareCopied] = useState(false)

  const { data: article, isLoading } = useSWR(["article", id], () => getArticleById(id))

  useEffect(() => {
    if (!article) return
    setLikes(article.likes)
    setLiked(article.liked ?? false)
  }, [article])

  useEffect(() => {
    void recordView(id)
  }, [id])

  if (isLoading && !article) return <LoadingState />
  if (!article) {
    return (
      <SiteShell>
        <Container className="py-20 text-center text-ink-soft">文章不存在</Container>
      </SiteShell>
    )
  }

  const headings = extractArticleHeadings(article.content)
  const wordCount = article.content.replace(/\s/g, "").length
  const readingMinutes = Math.max(1, Math.ceil(wordCount / 400))
  const cover = article.cover || "/kv/bocchi-lace.jpg"
  const articleId = article.id
  const summary = cleanArticleSummary(article.summary)

  async function handleLike() {
    if (!isLoggedIn) return
    const updated = await toggleArticleLike(articleId)
    if (updated) {
      setLikes(updated.likes)
      setLiked(updated.liked ?? false)
    }
  }

  async function handleShare() {
    const url = `${window.location.origin}/articles/${articleId}`
    try {
      await navigator.clipboard.writeText(url)
    } catch {
      const textarea = document.createElement("textarea")
      textarea.value = url
      document.body.appendChild(textarea)
      textarea.select()
      document.execCommand("copy")
      document.body.removeChild(textarea)
    }
    setShareCopied(true)
    window.setTimeout(() => setShareCopied(false), 2000)
  }

  return (
    <SiteShell immersiveHeader>
      <ReadingProgress />
      <ArticleCover
        title={article.title}
        cover={cover}
        createdAt={article.createdAt}
        views={article.views}
        readingMinutes={readingMinutes}
        category={article.tags[0]?.name || "技术随笔"}
      />

      <section className="bg-[var(--article-background)] transition-colors duration-300">
        <Container className="max-w-[1240px] pb-20 pt-8 sm:pt-12 lg:pb-28">
          <div className="grid items-start gap-10 lg:grid-cols-[250px_minmax(0,1fr)] lg:gap-14 xl:gap-20">
            <aside className="lg:sticky lg:top-20">
              <ArticleSidebar article={article} headings={headings} />
            </aside>

            <article className="min-w-0">
              <div className="mb-5 flex flex-wrap items-center justify-between gap-3 text-sm text-[var(--article-muted)]">
                <Link
                  href="/"
                  className="inline-flex items-center gap-2 transition-colors hover:text-[var(--article-accent)]"
                >
                  <ArrowLeft className="size-4" />
                  返回首页
                </Link>
                <button
                  type="button"
                  onClick={handleShare}
                  className="inline-flex items-center gap-2 transition-colors hover:text-[var(--article-accent)]"
                >
                  <Share2 className="size-4" />
                  {shareCopied ? "链接已复制" : "分享"}
                </button>
              </div>

              <div className="article-reader-surface rounded-xl px-5 py-6 shadow-[0_20px_60px_var(--article-surface-shadow)] sm:px-8 sm:py-9 lg:px-10">
                <div className="flex flex-wrap items-center gap-2 border-b border-[var(--article-divider)] pb-5">
                  {article.tags.map((tag) => (
                    <Link key={tag.id} href={`/search?q=${encodeURIComponent(tag.name)}&page=1`}>
                      <Badge className="rounded-full border border-[var(--article-accent-soft)] bg-[var(--article-accent-soft)] text-[var(--article-accent)] hover:bg-[var(--article-accent-soft)]">
                        {tag.name}
                      </Badge>
                    </Link>
                  ))}
                  <span className="ml-auto inline-flex items-center gap-1.5 text-xs text-[var(--article-faint)]">
                    <Clock3 className="size-3.5" />
                    {readingMinutes} min read
                  </span>
                </div>

                {summary && (
                  <section className="mt-6 rounded-lg border-l-4 border-[var(--article-accent)] bg-[var(--article-summary)] px-4 py-4 sm:px-5">
                    <p className="font-playful text-sm font-bold text-[var(--article-accent)]">摘要</p>
                    <p className="mt-2 text-sm leading-7 text-[var(--article-muted)]">{summary}</p>
                  </section>
                )}

                <div className="mt-8">
                  <ArticleContent content={article.content} images={article.images} />
                </div>

                <div className="mt-12 flex flex-wrap items-center gap-3 border-t border-[var(--article-divider)] pt-6">
                  <Button
                    variant={liked ? "default" : "outline"}
                    size="sm"
                    onClick={handleLike}
                    disabled={!isLoggedIn}
                    className={cn(
                      "rounded-full",
                      liked && "bg-[var(--article-accent)] text-white hover:bg-[var(--article-accent-hover)]",
                    )}
                  >
                    <ThumbsUp className={cn("size-4", liked && "fill-current")} />
                    <span>{formatNumber(likes)}</span>
                  </Button>
                  <Button variant="ghost" size="sm" onClick={handleShare} className="rounded-full">
                    {shareCopied ? <Copy className="size-4" /> : <Share2 className="size-4" />}
                    {shareCopied ? "已复制链接" : "复制链接"}
                  </Button>
                  <div className="ml-auto flex items-center gap-4 text-xs text-[var(--article-faint)]">
                    <span className="inline-flex items-center gap-1.5">
                      <Eye className="size-3.5" />
                      {formatNumber(article.views)}
                    </span>
                    <span>{article.commentsCount} 条评论</span>
                  </div>
                </div>

                <section id="comments" className="mt-14 border-t border-[var(--article-divider)] pt-8">
                  <p className="label-meta text-[var(--article-accent)]">community / responses</p>
                  <h2 className="font-playful mt-2 text-3xl font-bold text-[var(--article-text)]">
                    评论（{article.commentsCount}）
                  </h2>
                  <CommentList articleId={article.id} authorId={article.author.id} />
                </section>
              </div>
            </article>
          </div>
        </Container>
      </section>
    </SiteShell>
  )
}

function ArticleCover({
  title,
  cover,
  createdAt,
  views,
  readingMinutes,
  category,
}: {
  title: string
  cover: string
  createdAt: string
  views: number
  readingMinutes: number
  category: string
}) {
  return (
    <section data-article-cover className="relative h-[54svh] min-h-[500px] max-h-[720px] overflow-hidden bg-[#17171d]">
      <Image
        src={cover}
        alt=""
        fill
        priority
        sizes="100vw"
        className="object-cover"
        style={{ objectPosition: "center 48%" }}
      />
      <div className="absolute inset-0 bg-[linear-gradient(180deg,rgba(12,13,20,0.48),rgba(12,13,20,0.2)_48%,rgba(12,13,20,0.7))]" aria-hidden />
      <div className="relative z-10 flex h-full items-center justify-center px-5 pb-10 pt-16 text-center text-white sm:pb-16">
        <div className="max-w-5xl">
          <h1 className="max-w-[1150px] font-playful text-[clamp(2rem,3.6vw,3.75rem)] font-bold leading-tight tracking-wide [text-shadow:0_3px_16px_rgba(0,0,0,0.45)] lg:max-w-none lg:whitespace-nowrap">
            {title}
          </h1>
          <div className="mt-6 flex flex-wrap items-center justify-center gap-x-5 gap-y-2 text-xs text-white/85 sm:text-sm">
            <span className="inline-flex items-center gap-1.5">
              <CalendarDays className="size-4" />
              发表于 {formatDate(createdAt)}
            </span>
            <span className="inline-flex items-center gap-1.5">
              <Eye className="size-4" />
              {formatNumber(views)} 阅读
            </span>
            <span className="inline-flex items-center gap-1.5">
              <Clock3 className="size-4" />
              {readingMinutes} min read
            </span>
            <span className="inline-flex items-center gap-1.5">
              <Heart className="size-4" />
              {category}
            </span>
          </div>
        </div>
      </div>
      <div className="pointer-events-none absolute inset-x-0 bottom-0 z-20 h-20 sm:h-28" aria-hidden>
        <svg viewBox="0 0 1440 112" preserveAspectRatio="none" className="h-full w-full">
          <path d="M0 52C220 78 388 81 584 66C786 51 970 31 1152 52C1270 66 1354 72 1440 58V112H0Z" fill="var(--article-wave)" />
          <path d="M0 72C190 57 360 96 590 86C820 76 1004 45 1190 70C1290 84 1364 89 1440 80V112H0Z" fill="var(--article-background)" />
        </svg>
      </div>
    </section>
  )
}

function cleanArticleSummary(summary: string): string {
  return summary
    .replace(/```[\s\S]*?```/g, "")
    .replace(/!\[[^\]]*\]\([^)]*\)/g, "")
    .replace(/^#{1,6}\s+/gm, "")
    .replace(/[*_`~]/g, "")
    .replace(/\s+/g, " ")
    .trim()
    .slice(0, 220)
}

function ArticleSidebar({
  article,
  headings,
}: {
  article: NonNullable<Awaited<ReturnType<typeof getArticleById>>>
  headings: ReturnType<typeof extractArticleHeadings>
}) {
  return (
    <div className="article-sidebar text-center lg:text-left">
      <div className="mx-auto size-28 overflow-hidden rounded-full border-2 border-[var(--article-avatar-ring)] bg-[var(--article-image-surface)] shadow-[0_0_0_6px_var(--article-avatar-halo)] lg:mx-0">
        <Avatar src={article.author.avatar || "/kv/bocchi-sunglasses.jpg"} alt={article.author.username} size={112} className="size-full rounded-full border-0" />
      </div>
      <h2 className="font-playful mt-5 text-2xl font-bold text-[var(--article-text)]">{article.author.username}</h2>
      <p className="mt-3 text-sm leading-7 text-[var(--article-muted)]">
        热爱写代码，也热爱生活。
        <br />
        在技术与日常之间寻找灵感。
      </p>
      <div className="my-7 h-px bg-[var(--article-divider)]" aria-hidden />
      <ArticleToc headings={headings} />
    </div>
  )
}
