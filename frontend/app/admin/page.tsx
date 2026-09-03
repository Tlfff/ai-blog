"use client"

import { useEffect } from "react"
import Link from "next/link"
import { useRouter } from "next/navigation"
import useSWR from "swr"
import { LayoutDashboard, BookOpen, FileText, MessageSquare, Users, BarChart3, Trash2 } from "lucide-react"
import { Container } from "@/components/layout/container"
import { SiteShell } from "@/components/layout/site-shell"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { useAuth } from "@/hooks/use-auth"
import { getAdminArticleList } from "@/api/articles"
import { getAdminCommentCollection } from "@/api/comments"

export default function AdminPage() {
  const { isAdmin } = useAuth()
  const router = useRouter()

  useEffect(() => {
    if (!isAdmin) {
      router.push("/")
    }
  }, [isAdmin, router])

  if (!isAdmin) {
    return null
  }

  const { data: publishedArticles } = useSWR("admin-published", () => getAdminArticleList(3))
  const { data: draftArticles } = useSWR("admin-drafts", () => getAdminArticleList(2))
  const { data: deletedArticles } = useSWR("admin-deleted", () => getAdminArticleList(1))
  const { data: commentCollection } = useSWR(
    "admin-comment-collection",
    getAdminCommentCollection,
  )

  const totalArticles = (publishedArticles?.total || 0) + (draftArticles?.total || 0) + (deletedArticles?.total || 0)
  const draftCount = draftArticles?.total || 0
  const commentCount = commentCollection?.comments.length || 0

  const stats = [
    { label: "文章总数", value: String(totalArticles), icon: BookOpen, color: "text-primary" },
    { label: "草稿数量", value: String(draftCount), icon: FileText, color: "text-secondary" },
    { label: "评论总数", value: String(commentCount), icon: MessageSquare, color: "text-muted-foreground" },
    { label: "用户总数", value: "5", icon: Users, color: "text-destructive" },
  ]

  const menuItems = [
    { href: "/admin/articles", label: "文章管理", icon: BookOpen, desc: "管理所有文章（草稿/已发表）" },
    { href: "/admin/trash", label: "垃圾箱", icon: Trash2, desc: "管理已删除的文章" },
    { href: "/admin/comments", label: "评论管理", icon: MessageSquare, desc: "管理用户评论" },
  ]

  return (
    <SiteShell>
      <Container className="py-8 md:py-12">
        <div className="mb-8 flex flex-col gap-3 border-b border-border pb-6 md:flex-row md:items-end md:justify-between">
          <div>
            <p className="label-meta text-sakura-deep">control room / overview</p>
            <h1 className="title-display mt-2 flex items-center gap-3 text-4xl text-ink">
              <LayoutDashboard className="size-7 text-sakura-deep" />
              管理后台
            </h1>
          </div>
          <Link href="/" className="label-meta text-ink transition-colors hover:text-sakura-deep">
            back to site →
          </Link>
        </div>

        <div className="grid grid-cols-2 gap-4 md:grid-cols-4">
          {stats.map((stat) => (
            <Card key={stat.label} className="rounded-sm">
              <CardContent className="flex flex-col items-center py-5">
                <stat.icon className={`mb-2 size-5 ${stat.color}`} />
                <span className="title-display text-3xl text-ink">{stat.value}</span>
                <span className="label-meta mt-1 text-ink-soft">{stat.label}</span>
              </CardContent>
            </Card>
          ))}
        </div>

        <div className="mt-6">
          <Card className="mt-8 rounded-sm">
            <CardHeader>
              <CardTitle className="flex items-center gap-3">
                <BarChart3 className="size-4 text-sakura-deep" />
                <span className="title-display text-2xl text-ink">快捷操作</span>
              </CardTitle>
            </CardHeader>
            <CardContent>
              <div className="grid grid-cols-1 gap-3 md:grid-cols-3">
                {menuItems.map((item) => (
                  <Link key={item.href} href={item.href} className="group">
                    <div className="flex items-center gap-4 rounded-sm border border-border p-4 transition-colors hover:-translate-y-0.5 hover:border-sakura-deep hover:bg-sakura-wash">
                      <div className="flex size-10 items-center justify-center rounded-sm bg-sakura-wash">
                        <item.icon className="size-5 text-sakura-deep" />
                      </div>
                      <div>
                        <p className="font-medium group-hover:text-primary">{item.label}</p>
                        <p className="text-xs text-muted-foreground">{item.desc}</p>
                      </div>
                    </div>
                  </Link>
                ))}
              </div>
            </CardContent>
          </Card>
        </div>
      </Container>
    </SiteShell>
  )
}
