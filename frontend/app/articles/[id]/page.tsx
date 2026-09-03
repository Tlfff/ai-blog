"use client"

import { useEffect, use, useState } from "react"
import Link from "next/link"
import useSWR from "swr"
import { ArticleContent } from "@/components/article/article-content"
import { ArrowLeft, BookOpen, Clock3, Eye, Share2, ThumbsUp } from "lucide-react"
import { getArticleById, toggleArticleLike, recordView } from "@/api/articles"
import { Container } from "@/components/layout/container"
import { SiteShell } from "@/components/layout/site-shell"
import { Avatar } from "@/components/ui/avatar"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { formatDate, formatNumber, formatRelativeTime } from "@/lib/format"
import { CommentList } from "@/components/comment/comment-list"
import { useAuth } from "@/hooks/use-auth"
import { LoadingState } from "@/components/ui/spinner"
import { cn } from "@/lib/utils"

export default function ArticleDetailPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params)
  const { user, isLoggedIn } = useAuth()
  const [likes, setLikes] = useState(0)
  const [liked, setLiked] = useState(false)
  const [shareCopied, setShareCopied] = useState(false)

  const { data: article, isLoading } = useSWR(["article", id], () => getArticleById(id))

  useEffect(() => {
    if (article) {
      setLikes(article.likes)
      setLiked(article.liked ?? false)
    }
  }, [article])

  useEffect(() => {
    recordView(id)
  }, [id])

  if (isLoading && !article) return <LoadingState />
  if (!article) return <div className="py-12 text-center">文章不存在</div>

  async function handleLike() {
    if (!isLoggedIn || !article) return
    const updated = await toggleArticleLike(article.id)
    if (updated) {
      setLikes(updated.likes)
      setLiked(updated.liked ?? false)
    }
  }

  async function handleShare() {
    if (!article) return
    const url = `${window.location.origin}/articles/${article.id}`
    try {
      await navigator.clipboard.writeText(url)
      setShareCopied(true)
      setTimeout(() => setShareCopied(false), 2000)
    } catch {
      const textarea = document.createElement("textarea")
      textarea.value = url
      document.body.appendChild(textarea)
      textarea.select()
      document.execCommand("copy")
      document.body.removeChild(textarea)
      setShareCopied(true)
      setTimeout(() => setShareCopied(false), 2000)
    }
  }

  return (
    <SiteShell>
      <Container className="py-8 md:py-12">
        <div className="grid grid-cols-1 gap-10 lg:grid-cols-[minmax(0,1fr)_250px] lg:gap-14">
          <article className="prose-container max-w-none">
            <div className="flex flex-wrap items-center gap-3 text-sm text-ink-soft">
              <Link href="/" className="label-meta inline-flex items-center gap-2 text-ink transition-colors hover:text-sakura-deep">
                <ArrowLeft className="size-4" />
                <span>back to archive</span>
              </Link>
              <span className="text-border" aria-hidden>/</span>
              <span className="label-meta">{formatRelativeTime(article.createdAt)}</span>
              {article.createdAt !== article.updatedAt && (
                <>
                  <span className="text-border" aria-hidden>/</span>
                  <span className="label-meta">更新于 {formatDate(article.updatedAt)}</span>
                </>
              )}
            </div>

            <div className="mt-7 border-y border-border py-7 md:py-9">
              <p className="label-meta text-sakura-deep">NO. {String(article.id).padStart(3, "0")} / FEATURED NOTE</p>
              <h1 className="title-display mt-4 max-w-4xl text-4xl leading-[1.08] text-ink md:text-6xl">
                {article.title}
              </h1>
              {article.summary && (
                <p className="mt-5 max-w-2xl text-base leading-8 text-ink-soft md:text-lg">{article.summary}</p>
              )}
            </div>

            <div className="mt-6 flex flex-wrap items-center gap-4">
              <Link href={`/profile?userId=${article.author.id}`} className="flex items-center gap-2 group">
                <div className="relative">
                  <Avatar src={article.author.avatar} alt={article.author.username} size={44} />
                  <div className="absolute -bottom-0.5 -right-0.5 flex size-5 items-center justify-center rounded-full bg-card border-2 border-background">
                    <div className="size-2 rounded-full bg-green-500" />
                  </div>
                </div>
                <div>
                  <p className="text-sm font-medium group-hover:text-primary transition-colors">{article.author.username}</p>
                  <p className="text-xs text-muted-foreground">{article.author.location}</p>
                </div>
              </Link>

              <div className="flex flex-wrap gap-2">
                {article.tags.map((tag) => (
                  <Link key={tag.id} href={`/search?q=${encodeURIComponent(tag.name)}&page=1`}>
                    <Badge className="border border-border bg-transparent hover:border-sakura-deep hover:text-sakura-deep transition-colors">
                      {tag.name}
                    </Badge>
                  </Link>
                ))}
              </div>
            </div>

            <div className="relative mt-8 overflow-hidden rounded-sm border border-border bg-paper-deep shadow-[8px_8px_0_var(--paper-deep)]">
              <img
                src={article.cover || "/kv/bocchi-lace.jpg"}
                alt={article.title}
                className="max-h-[30rem] w-full object-cover"
              />
              <span className="label-meta absolute bottom-3 left-3 bg-paper px-3 py-2 text-ink">visual note / 01</span>
            </div>

            <div className="mt-8 flex flex-wrap items-center gap-4 border-b border-border pb-6">
              <Button
                variant={liked ? "default" : "outline"}
                size="sm"
                onClick={handleLike}
                disabled={!isLoggedIn}
                className={liked ? "bg-sakura-deep text-primary-foreground" : ""}
              >
                <ThumbsUp className={cn("size-4", liked && "fill-current")} />
                <span className="ml-1.5">{formatNumber(likes)}</span>
              </Button>

              <Button
                variant="ghost"
                size="sm"
                onClick={handleShare}
                className=""
              >
                <Share2 className="size-4" />
                <span className="ml-1.5">{shareCopied ? "已复制" : "分享"}</span>
              </Button>

              <div className="flex flex-1 items-center justify-end gap-4 text-xs text-ink-soft sm:gap-6">
                <span>{formatNumber(article.views)} 次阅读</span>
                <span>{article.commentsCount} 条评论</span>
              </div>
            </div>

            <div className="mt-8">
              <ArticleContent content={article.content} images={article.images} />
            </div>

            <section id="comments" className="mt-16 border-t border-border pt-8">
              <p className="label-meta text-sakura-deep">community / responses</p>
              <h2 className="title-display mt-2 text-3xl text-ink">评论 ({article.commentsCount})</h2>
              <CommentList articleId={article.id} authorId={article.author.id} />
            </section>
          </article>

          <aside className="hidden lg:block">
            <div className="sticky top-24 space-y-5">
              <div className="border-y border-border py-5">
                <p className="label-meta text-sakura-deep">article index</p>
                <div className="mt-4 space-y-4 text-sm text-ink-soft">
                  <div className="flex items-center justify-between gap-3">
                    <span className="inline-flex items-center gap-2"><BookOpen className="size-4 text-sakura-deep" />文章状态</span>
                    <span className="text-ink">已发布</span>
                  </div>
                  <div className="flex items-center justify-between gap-3">
                    <span className="inline-flex items-center gap-2"><Clock3 className="size-4 text-sakura-deep" />发布于</span>
                    <span className="text-ink">{formatDate(article.createdAt)}</span>
                  </div>
                  <div className="flex items-center justify-between gap-3">
                    <span className="inline-flex items-center gap-2"><Eye className="size-4 text-sakura-deep" />阅读量</span>
                    <span className="text-ink">{formatNumber(article.views)}</span>
                  </div>
                </div>
              </div>
              <div className="bg-sakura-wash p-5">
                <p className="label-meta text-sakura-deep">keep reading</p>
                <p className="mt-3 text-sm leading-7 text-ink-soft">
                  如果这篇记录对你有帮助，欢迎留下想法，也可以把它分享给同样喜欢探索的人。
                </p>
                <Link href="#comments" className="mt-4 inline-flex text-sm font-medium text-ink underline decoration-sakura decoration-2 underline-offset-4 hover:text-sakura-deep">
                  写下评论 →
                </Link>
              </div>
            </div>
          </aside>
        </div>
      </Container>
    </SiteShell>
  )
}
