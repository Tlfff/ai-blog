"use client"

import { SITE_IMAGES } from "@/lib/site-images"
import Image from "next/image"
import Link from "next/link"
import { ArrowLeft, Check, Code2, Moon, Sun } from "lucide-react"
import { cn } from "@/lib/utils"

type AuthVariant = "login" | "register"

const AUTH_CONFIG = {
  login: {
    image: SITE_IMAGES.pages.primarySky,
    imagePosition: "object-[center_42%]",
    eyebrow: "welcome back / 登录",
    heroTitle: "欢迎回来，\n继续你的阅读轨迹。",
    heroDescription: "登录后，个人资料、通知与浏览历史都会回到原来的位置。",
    noteLabel: "your space is ready",
    noteTitle: "从上次停下的地方继续",
    noteItems: ["账号会自动保留登录状态", "支持日间与深夜模式"],
  },
  register: {
    image: SITE_IMAGES.pages.registerHero,
    imagePosition: "object-[center_38%]",
    eyebrow: "new chapter / 注册",
    heroTitle: "创建一个账号，\n拥有自己的小小空间。",
    heroDescription: "留下昵称，保存浏览足迹，也及时收到新的互动。",
    noteLabel: "a space of your own",
    noteTitle: "资料、通知与阅读足迹",
    noteItems: ["昵称构成你的站内身份", "手机号用于登录"],
  },
} as const

export function AuthShell({ variant, children }: { variant: AuthVariant; children: React.ReactNode }) {
  const config = AUTH_CONFIG[variant]

  return (
    <main className={cn("auth-page min-h-screen bg-[var(--auth-background)] text-[var(--auth-ink)]", `auth-${variant}`)}>
      <div className={cn("grid min-h-screen", variant === "register" ? "lg:grid-cols-[54%_46%]" : "lg:grid-cols-[58%_42%]")}>
        <section className="auth-visual relative isolate min-h-[300px] overflow-hidden text-white sm:min-h-[360px] lg:min-h-screen">
          <Image src={config.image} alt="" fill priority sizes="(min-width: 1024px) 58vw, 100vw" className={cn("object-cover", config.imagePosition)} />
          <div className={cn("absolute inset-0", variant === "register" ? "bg-[linear-gradient(105deg,rgba(12,65,86,0.82),rgba(33,142,153,0.56),rgba(57,169,146,0.28))]" : "bg-[linear-gradient(105deg,rgba(9,57,91,0.84),rgba(20,125,166,0.54),rgba(47,168,164,0.3))]")} aria-hidden />
          <div className="absolute inset-0 bg-[radial-gradient(circle_at_80%_5%,rgba(255,255,255,0.18),transparent_30%)]" aria-hidden />
          <div className="absolute inset-x-0 bottom-0 h-2/3 bg-gradient-to-t from-[#123d55]/90 via-[#174f65]/40 to-transparent" aria-hidden />

          <div className="relative flex min-h-[300px] flex-col px-5 py-5 sm:min-h-[360px] sm:px-8 sm:py-7 lg:min-h-screen lg:px-[clamp(3rem,7vw,7rem)] lg:py-8">
            <div className="flex items-center justify-between gap-4">
              <Link href="/" className="flex items-center gap-2.5"><span className="grid size-10 place-items-center rounded-xl border border-white/28 bg-white/12 backdrop-blur-sm"><Code2 className="size-5" /></span><span className="font-playful text-xl font-bold sm:text-2xl">Link start！</span></Link>
              <Link href="/" className="inline-flex items-center gap-2 rounded-full border border-white/28 bg-[#143b56]/20 px-3 py-2 text-xs font-semibold text-white backdrop-blur-sm hover:bg-[#143b56]/35"><ArrowLeft className="size-4" />返回首页</Link>
            </div>

            <div className="mt-auto max-w-[680px] pb-8 pt-14 sm:pb-10 lg:pb-[12vh]">
              <p className="font-mono text-[0.62rem] font-semibold uppercase tracking-[0.24em] text-white/78">{config.eyebrow}</p>
              <h1 className="mt-4 whitespace-pre-line font-playful text-[clamp(2.35rem,4.2vw,4rem)] font-bold leading-[1.08] tracking-[-0.045em] [text-shadow:0_3px_20px_rgba(0,30,50,0.3)]">{config.heroTitle}</h1>
              <p className="mt-5 max-w-xl text-sm leading-7 text-white/86 sm:text-base">{config.heroDescription}</p>

              <div className="mt-7 hidden max-w-md rounded-[1.15rem] border border-white/24 bg-[#123b54]/24 p-5 backdrop-blur-sm sm:block lg:mt-10">
                <p className="font-mono text-[0.58rem] uppercase tracking-[0.18em] text-white/68">{config.noteLabel}</p>
                <h2 className="mt-2 font-playful text-xl font-bold">{config.noteTitle}</h2>
                <div className="mt-4 flex flex-wrap gap-x-5 gap-y-2 border-t border-white/15 pt-4">
                  {config.noteItems.map((item, index) => <span key={item} className="inline-flex items-center gap-2 text-xs text-white/76"><span className={cn("size-2 rounded-full", index === 0 ? "bg-[var(--auth-yellow)]" : "bg-[var(--auth-teal)]")} />{item}</span>)}
                </div>
              </div>
            </div>
          </div>
        </section>

        <section className="relative flex min-h-[632px] items-center justify-center bg-[var(--auth-background)] px-4 pb-10 pt-0 sm:px-8 lg:min-h-screen lg:px-[clamp(3rem,7vw,7rem)] lg:py-16">
          <div className="absolute right-5 top-5 z-10 sm:right-8 sm:top-7"><AuthThemeToggle /></div>
          <div className="auth-form-card relative z-10 -mt-8 w-full max-w-[470px] rounded-[1.35rem] border border-[var(--auth-border)] bg-[var(--auth-card)] p-6 shadow-[0_22px_60px_var(--auth-shadow)] backdrop-blur-xl sm:-mt-12 sm:p-8 lg:mt-0 lg:border-0 lg:bg-transparent lg:p-0 lg:shadow-none lg:backdrop-blur-none">
            {children}
          </div>
        </section>
      </div>
    </main>
  )
}

export function AuthField({ label, icon, action, children, hint }: { label: string; icon?: React.ReactNode; action?: React.ReactNode; children: React.ReactNode; hint?: string }) {
  return (
    <label className="block">
      <span className="auth-label flex items-center justify-between gap-3 text-sm font-semibold"><span>{label}</span>{action}</span>
      <span className="relative mt-2 block">{icon ? <span className="pointer-events-none absolute left-4 top-1/2 z-10 -translate-y-1/2 text-[var(--auth-faint)]">{icon}</span> : null}{children}</span>
      {hint ? <span className="mt-1.5 block text-xs text-[var(--auth-faint)]">{hint}</span> : null}
    </label>
  )
}

export function AuthNotice({ tone = "teal", title, children }: { tone?: "teal" | "yellow"; title: string; children: React.ReactNode }) {
  return (
    <div className={cn("flex gap-3 rounded-2xl p-4", tone === "teal" ? "bg-[var(--auth-teal-soft)] text-[var(--auth-teal-deep)]" : "bg-[var(--auth-yellow-soft)] text-[var(--auth-yellow-deep)]")}>
      <span className="mt-0.5 grid size-7 shrink-0 place-items-center rounded-full bg-white/45"><Check className="size-4" /></span>
      <div><p className="text-xs font-bold">{title}</p><p className="mt-1 text-xs leading-5 opacity-80">{children}</p></div>
    </div>
  )
}

export const authInputClass = "h-12 w-full rounded-xl border border-[var(--auth-border-strong)] bg-[var(--auth-input)] px-4 text-sm text-[var(--auth-ink)] outline-none transition placeholder:text-[var(--auth-faint)] focus:border-[var(--auth-sky)] focus:ring-4 focus:ring-[var(--auth-sky-soft)] disabled:cursor-not-allowed disabled:opacity-60"

function AuthThemeToggle() {
  function toggleTheme() {
    const root = document.documentElement
    const nextDark = !root.classList.contains("dark")
    root.classList.toggle("dark", nextDark)
    root.dataset.theme = nextDark ? "dark" : "light"
    localStorage.setItem("site-theme", nextDark ? "dark" : "light")
  }

  return <button type="button" onClick={toggleTheme} aria-label="切换日间或深夜模式" className="grid size-10 place-items-center rounded-full bg-[var(--auth-card)] text-[var(--auth-lavender)] shadow-[0_8px_22px_var(--auth-shadow)] hover:bg-[var(--auth-lavender-soft)]"><Moon className="size-5 dark:hidden" /><Sun className="hidden size-5 dark:block" /></button>
}
