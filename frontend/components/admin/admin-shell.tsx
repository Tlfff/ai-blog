"use client"

import { useEffect } from "react"
import Link from "next/link"
import { usePathname, useRouter } from "next/navigation"
import {
  ArrowLeft,
  BookOpen,
  Code2,
  FileText,
  LayoutDashboard,
  MessageSquare,
  Moon,
  PenLine,
  Sun,
  Trash2,
} from "lucide-react"
import { Avatar } from "@/components/ui/avatar"
import { useAuth } from "@/hooks/use-auth"
import { cn } from "@/lib/utils"

const NAV_ITEMS = [
  { href: "/admin", label: "总览", shortLabel: "总览", icon: LayoutDashboard },
  { href: "/admin/articles", label: "文章管理", shortLabel: "文章", icon: BookOpen },
  { href: "/admin/drafts", label: "草稿箱", shortLabel: "草稿", icon: FileText },
  { href: "/admin/comments", label: "评论管理", shortLabel: "评论", icon: MessageSquare },
  { href: "/admin/trash", label: "回收站", shortLabel: "回收站", icon: Trash2 },
] as const

export function AdminShell({ children }: { children: React.ReactNode }) {
  const pathname = usePathname()
  const router = useRouter()
  const { user, isAdmin } = useAuth()

  useEffect(() => {
    if (!isAdmin) router.replace("/")
  }, [isAdmin, router])

  if (!isAdmin || !user) return null

  return (
    <div className="admin-page min-h-screen bg-[var(--admin-background)] text-[var(--admin-ink)] lg:grid lg:grid-cols-[250px_minmax(0,1fr)]">
      <aside className="admin-rail hidden h-screen flex-col overflow-y-auto bg-[var(--admin-rail)] px-5 py-6 text-white lg:sticky lg:top-0 lg:flex">
        <Link href="/" className="flex items-center gap-3">
          <span className="grid size-10 place-items-center rounded-xl border border-white/20 bg-white/10">
            <Code2 className="size-5" />
          </span>
          <span className="font-playful text-xl font-bold">Link start！</span>
        </Link>
        <p className="mt-6 font-mono text-[0.58rem] font-semibold uppercase tracking-[0.24em] text-[var(--admin-teal)]">admin / control room</p>

        <div className="mt-7 flex items-center gap-3">
          <Avatar src={user.avatar} alt={user.username} size={52} className="border-2 border-white/30" />
          <div className="min-w-0">
            <p className="truncate font-playful text-lg font-bold">{user.username}</p>
            <p className="mt-1 inline-flex items-center gap-1.5 rounded-full bg-[var(--admin-teal)]/12 px-2 py-1 text-[0.65rem] text-[#a8e8df]">
              <span className="size-1.5 rounded-full bg-[var(--admin-teal)]" />管理员
            </p>
          </div>
        </div>

        <p className="mb-3 mt-10 font-mono text-[0.57rem] font-semibold uppercase tracking-[0.22em] text-white/38">workspace</p>
        <nav className="space-y-1.5" aria-label="后台导航">
          {NAV_ITEMS.map((item) => {
            const active = isActivePath(pathname, item.href)
            const Icon = item.icon
            return (
              <Link
                key={item.href}
                href={item.href}
                className={cn(
                  "flex items-center gap-3 rounded-xl px-3 py-3 text-sm font-semibold transition-colors",
                  active ? "bg-[#eaf7fb] text-[#28749e]" : "text-white/72 hover:bg-white/8 hover:text-white",
                )}
              >
                <Icon className="size-4" />
                {item.label}
              </Link>
            )
          })}
        </nav>

        <p className="mb-3 mt-8 font-mono text-[0.57rem] font-semibold uppercase tracking-[0.22em] text-white/38">quick action</p>
        <Link href="/editor" className="flex items-center gap-3 rounded-xl bg-[var(--admin-sky)] px-3 py-3 text-sm font-bold text-white shadow-lg shadow-black/10 hover:bg-[var(--admin-sky-deep)]">
          <PenLine className="size-4" />
          写文章
          <span className="ml-auto">›</span>
        </Link>

        <div className="mt-5">
          <AdminThemeButton rail />
        </div>

        <div className="mt-auto border-t border-white/10 pt-5">
          <Link href="/" className="flex items-center gap-3 rounded-xl px-3 py-2.5 text-sm text-white/70 hover:bg-white/8 hover:text-white">
            <ArrowLeft className="size-4" />返回博客
          </Link>
          <p className="mt-8 font-mono text-[0.55rem] uppercase tracking-[0.18em] text-white/30">system status</p>
          <p className="mt-2 flex items-center gap-2 text-xs text-white/52"><span className="size-2 rounded-full bg-[var(--admin-teal)]" />服务运行正常</p>
        </div>
      </aside>

      <div className="min-w-0">
        <header className="admin-command-bar sticky top-0 z-40 flex h-16 items-center border-b border-[var(--admin-border)] bg-[var(--admin-surface)]/92 px-4 backdrop-blur-xl sm:px-6 lg:h-[76px] lg:px-9">
          <div className="flex items-center gap-3 lg:hidden">
            <Link href="/" className="grid size-9 place-items-center rounded-xl bg-[var(--admin-navy)] text-white"><Code2 className="size-4" /></Link>
            <strong className="font-playful text-lg text-[var(--admin-ink)]">管理后台</strong>
          </div>
          <div className="hidden lg:block">
            <p className="font-mono text-[0.58rem] font-semibold uppercase tracking-[0.22em] text-[var(--admin-sky-deep)]">control room / {getSectionName(pathname)}</p>
            <p className="mt-1 font-playful text-lg font-bold text-[var(--admin-ink)]">管理后台</p>
          </div>

          <p className="ml-auto hidden font-mono text-[0.58rem] uppercase tracking-[0.18em] text-[var(--admin-faint)] sm:block">content management workspace</p>
          <div className="ml-2 sm:ml-5"><AdminThemeButton /></div>
          <Avatar src={user.avatar} alt={user.username} size={38} className="ml-2 border-2 border-[var(--admin-surface)]" />
        </header>

        <nav className="admin-mobile-nav flex gap-1 overflow-x-auto border-b border-[var(--admin-border)] bg-[var(--admin-surface)] px-3 py-2 lg:hidden" aria-label="后台快捷导航">
          {NAV_ITEMS.map((item) => {
            const active = isActivePath(pathname, item.href)
            return <Link key={item.href} href={item.href} className={cn("shrink-0 rounded-xl px-3 py-2 text-xs font-semibold", active ? "bg-[var(--admin-sky-soft)] text-[var(--admin-sky-deep)]" : "text-[var(--admin-muted)]")}>{item.shortLabel}</Link>
          })}
          <Link href="/editor" className="ml-auto shrink-0 rounded-full bg-[var(--admin-sky)] px-3 py-2 text-xs font-bold text-white">写文章</Link>
        </nav>

        <main className="p-4 sm:p-6 lg:p-9">{children}</main>
      </div>
    </div>
  )
}

function AdminThemeButton({ rail = false }: { rail?: boolean }) {
  function toggleTheme() {
    const root = document.documentElement
    const nextDark = !root.classList.contains("dark")
    root.classList.toggle("dark", nextDark)
    root.dataset.theme = nextDark ? "dark" : "light"
    localStorage.setItem("site-theme", nextDark ? "dark" : "light")
  }

  return (
    <button type="button" onClick={toggleTheme} aria-label="切换后台日间或深夜模式" className={cn("grid size-9 place-items-center rounded-full transition-colors", rail ? "text-white/70 hover:bg-white/10 hover:text-white" : "text-[var(--admin-muted)] hover:bg-[var(--admin-lavender-soft)] hover:text-[var(--admin-lavender)]")}>
      <Moon className="size-4 dark:hidden" />
      <Sun className="hidden size-4 dark:block" />
    </button>
  )
}

function isActivePath(pathname: string, href: string) {
  return href === "/admin" ? pathname === href : pathname.startsWith(href)
}

function getSectionName(pathname: string) {
  return NAV_ITEMS.find((item) => isActivePath(pathname, item.href))?.shortLabel.toLowerCase() ?? "overview"
}
