"use client"

import { SITE_IMAGES } from "@/lib/site-images"
import Image from "next/image"
import Link from "next/link"
import { ArrowLeft, Code2, Home, Moon, RefreshCw, Search, Sun, TriangleAlert } from "lucide-react"
import { cn } from "@/lib/utils"

export type SystemStateVariant = "not-found" | "error" | "loading"

const STATE_CONFIG = {
  "not-found": {
    image: SITE_IMAGES.pages.primarySky,
    eyebrow: "route not found",
    title: "这条路没有通向文章。",
    description: "地址可能已经改变，或者页面从未存在过。",
    tone: "sky",
  },
  error: {
    image: SITE_IMAGES.pages.secondarySky,
    eyebrow: "system error / 500",
    title: "页面暂时出了点问题。",
    description: "刚才的内容没有顺利加载，请稍后再试。",
    tone: "lavender",
  },
  loading: {
    image: SITE_IMAGES.pages.primarySky,
    eyebrow: "loading / 01",
    title: "正在把内容带回来。",
    description: "先喝口水，页面很快就会准备好。",
    tone: "sky",
  },
} as const

export function SystemState({ variant, reset }: { variant: SystemStateVariant; reset?: () => void }) {
  const config = STATE_CONFIG[variant]
  const isLoading = variant === "loading"
  const isError = variant === "error"

  return (
    <main className={cn("system-state-page min-h-screen overflow-hidden bg-[var(--system-background)] text-[var(--system-ink)]", `system-state-${variant}`)}>
      <section className="relative isolate min-h-[410px] overflow-hidden text-white sm:min-h-[470px]">
        <Image src={config.image} alt="" fill priority sizes="100vw" className={cn("object-cover", isError ? "object-[center_42%]" : "object-[center_28%]")} />
        <div className={cn("absolute inset-0", isError ? "bg-[linear-gradient(90deg,rgba(31,63,105,0.9),rgba(65,103,158,0.62),rgba(104,91,157,0.5))]" : "bg-[linear-gradient(90deg,rgba(9,61,96,0.86),rgba(25,130,165,0.57),rgba(48,166,157,0.34))]")} aria-hidden />
        <div className="absolute inset-x-0 bottom-0 h-44 bg-gradient-to-t from-[#153f58]/70 to-transparent" aria-hidden />

        <header className="relative z-10 flex h-16 items-center justify-between px-5 sm:px-8 lg:px-12">
          <Link href="/" className="flex items-center gap-2.5"><span className="grid size-9 place-items-center rounded-xl border border-white/30 bg-white/12 backdrop-blur-sm"><Code2 className="size-5" /></span><span className="font-playful text-xl font-bold">Link start！</span></Link>
          <div className="flex items-center gap-2"><Link href="/" className="hidden items-center gap-2 rounded-full border border-white/28 bg-[#143b56]/20 px-4 py-2 text-xs font-semibold text-white backdrop-blur-sm hover:bg-[#143b56]/35 sm:inline-flex"><ArrowLeft className="size-4" />返回首页</Link><ThemeToggle /></div>
        </header>

        <div className="relative z-10 mx-auto flex max-w-6xl flex-col items-center px-5 pb-24 pt-16 text-center sm:pt-20">
          <p className="font-mono text-[0.62rem] font-semibold uppercase tracking-[0.25em] text-white/78">{config.eyebrow}</p>
          {variant === "not-found" ? <p className="mt-4 font-playful text-[clamp(6rem,16vw,12rem)] font-bold leading-none tracking-[-0.08em] text-white/17 [text-shadow:0_3px_18px_rgba(0,30,50,0.12)]">404</p> : <span className={cn("mt-5 grid size-16 place-items-center rounded-3xl border border-white/24 bg-[#173b5c]/22 backdrop-blur-sm", isError ? "text-[#ffd1ca]" : "text-[#dffbff]")}>{isError ? <TriangleAlert className="size-8" /> : <RefreshCw className={cn("size-8", isLoading && "animate-spin")} />}</span>}
          <h1 className={cn("font-playful font-bold leading-tight tracking-[-0.045em] text-white [text-shadow:0_3px_18px_rgba(0,30,50,0.24)]", variant === "not-found" ? "-mt-8 text-[clamp(2rem,4vw,3.5rem)]" : "mt-5 text-[clamp(2rem,4vw,3.5rem)]")}>{config.title}</h1>
          <p className="mt-4 max-w-xl text-sm leading-7 text-white/84 sm:text-base">{config.description}</p>
        </div>

        <div className="pointer-events-none absolute inset-x-0 bottom-0 z-20 h-20 sm:h-28" aria-hidden>
          <svg viewBox="0 0 1440 112" preserveAspectRatio="none" className="h-full w-full"><path d="M0 48C210 82 398 77 590 61C815 42 994 27 1168 51C1285 67 1366 70 1440 60V112H0Z" fill="var(--system-wave)" /><path d="M0 72C188 58 353 99 588 87C818 75 1002 47 1190 70C1290 82 1365 88 1440 80V112H0Z" fill="var(--system-background)" /></svg>
        </div>
      </section>

      <section className="relative z-30 -mt-12 px-4 pb-20 sm:-mt-16 sm:px-6">
        <div className={cn("mx-auto w-full rounded-[1.4rem] border border-[var(--system-border)] bg-[var(--system-card)] p-6 text-center shadow-[0_24px_70px_var(--system-shadow)] sm:p-10", variant === "not-found" ? "max-w-2xl" : "max-w-xl")}>
          {isLoading ? <LoadingSkeleton /> : <>
            <p className="font-mono text-[0.6rem] font-semibold uppercase tracking-[0.2em] text-[var(--system-accent)]">{isError ? "try again" : "lost page / 404"}</p>
            <p className="mt-4 text-sm leading-7 text-[var(--system-muted)]">{isError ? "你可以重新加载当前页面；如果问题持续，再回到首页继续浏览。" : "这个地址没有对应的公开页面，你可以从首页或文章搜索重新开始。"}</p>
            <div className="mt-7 flex flex-col justify-center gap-3 sm:flex-row">
              {isError && reset ? <button type="button" onClick={reset} className="inline-flex h-11 items-center justify-center gap-2 rounded-full bg-[var(--system-navy)] px-6 text-sm font-semibold text-white transition hover:bg-[var(--system-accent)]"><RefreshCw className="size-4" />重新加载</button> : null}
              {variant === "not-found" ? <><Link href="/" className="inline-flex h-11 items-center justify-center gap-2 rounded-full bg-[var(--system-navy)] px-6 text-sm font-semibold text-white transition hover:bg-[var(--system-accent)]"><Home className="size-4" />返回首页</Link><Link href="/search" className="inline-flex h-11 items-center justify-center gap-2 rounded-full bg-[var(--system-sky-soft)] px-6 text-sm font-semibold text-[var(--system-accent)] transition hover:bg-[var(--system-accent)] hover:text-white"><Search className="size-4" />搜索文章</Link></> : <Link href="/" className="inline-flex h-11 items-center justify-center gap-2 rounded-full bg-[var(--system-sky-soft)] px-6 text-sm font-semibold text-[var(--system-accent)] transition hover:bg-[var(--system-accent)] hover:text-white"><ArrowLeft className="size-4" />返回首页</Link>}
            </div>
          </>}
          {!isLoading ? <button type="button" onClick={() => window.history.back()} className="mt-7 inline-flex items-center gap-2 border-t border-[var(--system-border)] px-6 pt-5 text-xs text-[var(--system-faint)] hover:text-[var(--system-accent)]"><ArrowLeft className="size-3.5" />返回上一页，继续刚才的浏览</button> : null}
        </div>
      </section>
    </main>
  )
}

function LoadingSkeleton() {
  return <div className="space-y-5 text-left"><div className="mx-auto h-3 w-28 animate-pulse rounded-full bg-[var(--system-skeleton)]" /><div className="h-4 animate-pulse rounded-full bg-[var(--system-skeleton)]" /><div className="h-4 w-5/6 animate-pulse rounded-full bg-[var(--system-skeleton)]" /><div className="h-11 animate-pulse rounded-full bg-[var(--system-skeleton)]" /><div className="h-3 w-2/3 animate-pulse rounded-full bg-[var(--system-skeleton)]" /></div>
}

function ThemeToggle() {
  function toggleTheme() {
    const root = document.documentElement
    const dark = !root.classList.contains("dark")
    root.classList.toggle("dark", dark)
    root.dataset.theme = dark ? "dark" : "light"
    localStorage.setItem("site-theme", dark ? "dark" : "light")
  }

  return <button type="button" onClick={toggleTheme} aria-label="切换日间或深夜模式" className="grid size-9 place-items-center rounded-full border border-white/24 bg-white/12 text-white backdrop-blur-sm hover:bg-white/20"><Moon className="size-4 dark:hidden" /><Sun className="hidden size-4 dark:block" /></button>
}
