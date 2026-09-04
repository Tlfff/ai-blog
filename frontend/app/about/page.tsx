"use client"

import useSWR from "swr"
import {
  ArrowDown,
  ArrowUpRight,
  Code2,
  Compass,
  MapPin,
  Palette,
  Sparkles,
} from "lucide-react"
import { getStats } from "@/api/articles"
import { Container } from "@/components/layout/container"
import { SiteShell } from "@/components/layout/site-shell"
import { Avatar } from "@/components/ui/avatar"
import { Badge } from "@/components/ui/badge"
import { formatNumber } from "@/lib/format"

const FOCUS_ITEMS = ["界面与体验", "React", "Go", "生活记录"]

const TOPIC_ITEMS = [
  {
    title: "技术实践",
    description: "框架、工具与真实项目里的取舍。",
    icon: Code2,
    tone: "teal",
  },
  {
    title: "界面与产品",
    description: "颜色、交互和让人愿意继续使用的细节。",
    icon: Palette,
    tone: "sky",
  },
  {
    title: "日常与灵感",
    description: "阅读、观察，以及生活里不经意的发现。",
    icon: Sparkles,
    tone: "coral",
  },
] as const

export default function AboutPage() {
  const { data: stats } = useSWR("profile-stats", getStats)

  return (
    <SiteShell immersiveHeader>
      <div className="profile-page overflow-hidden">
        <section className="profile-hero relative isolate min-h-[610px] overflow-hidden text-white sm:min-h-[660px]">
          <div className="absolute inset-0 bg-[url('/kv/bq-3.png')] bg-cover bg-center" aria-hidden />
          <div className="absolute inset-0 bg-[linear-gradient(90deg,rgba(8,42,64,0.88)_0%,rgba(8,42,64,0.42)_52%,rgba(8,42,64,0.12)_100%)]" aria-hidden />
          <div className="absolute inset-x-0 bottom-0 h-56 bg-gradient-to-t from-[#082a40]/80 to-transparent" aria-hidden />

          <Container className="relative flex min-h-[610px] items-end pb-28 pt-28 sm:min-h-[660px] sm:pb-32">
            <div className="max-w-3xl">
              <p className="font-mono text-[0.68rem] font-medium uppercase tracking-[0.28em] text-[#b8f2e7]">
                about / author profile
              </p>
              <h1 className="mt-5 max-w-3xl font-playful text-[clamp(3.3rem,8vw,6.6rem)] font-bold leading-[0.98] tracking-[-0.06em] [text-shadow:0_4px_24px_rgba(0,0,0,0.25)]">
                认识作者，
                <br />
                也认识这个空间。
              </h1>
              <p className="mt-7 max-w-xl text-base leading-8 text-white/82 sm:text-lg">
                一个只有一个作者的博客，不需要重复展示文章，
                <br className="hidden sm:block" />
                只需要留下这个作者为什么持续记录。
              </p>
              <a
                href="#about-author"
                className="mt-8 inline-flex items-center gap-2 rounded-full border border-white/45 bg-white/10 px-4 py-2 text-sm font-medium text-white backdrop-blur-sm transition-colors hover:bg-white/20"
              >
                向下了解作者
                <ArrowDown className="size-4" />
              </a>
            </div>
          </Container>

          <div className="profile-hero-wave absolute -bottom-px left-[-5%] h-24 w-[110%] bg-[var(--profile-background)]" aria-hidden />
        </section>

        <section className="relative bg-[var(--profile-background)] pb-16 sm:pb-24">
          <Container>
            <div className="profile-identity relative z-10 -mt-3 grid gap-6 rounded-[1.5rem] border border-[var(--profile-border)] bg-[var(--profile-card)] p-5 shadow-[0_18px_55px_var(--profile-shadow)] sm:grid-cols-[auto_1fr_auto] sm:items-center sm:gap-8 sm:p-8">
              <div className="relative mx-auto sm:mx-0">
                <span className="absolute -inset-2 rounded-full border border-[var(--profile-teal)]/45 bg-[var(--profile-teal)]/10" aria-hidden />
                <Avatar src="/kv/bocchi-sunglasses.jpg" alt="睦子米" size={126} className="relative border-4 border-white shadow-[0_8px_24px_rgba(22,96,117,0.18)] dark:border-[var(--profile-card)]" />
              </div>

              <div className="min-w-0 text-center sm:text-left">
                <p className="font-mono text-[0.68rem] uppercase tracking-[0.24em] text-[var(--profile-teal-deep)]">
                  creator / muzimi
                </p>
                <div className="mt-2 flex flex-wrap items-center justify-center gap-2 sm:justify-start">
                  <h2 className="font-playful text-3xl font-bold tracking-[-0.035em] text-[var(--profile-ink)] sm:text-4xl">
                    睦子米
                  </h2>
                  <Badge className="rounded-full border-0 bg-[var(--profile-teal-wash)] text-[var(--profile-teal-deep)]">作者</Badge>
                </div>
                <p className="mt-3 text-sm leading-7 text-[var(--profile-muted)] sm:text-base">
                  写代码，也写下生活里值得留下的部分。
                </p>
                <div className="mt-3 flex flex-wrap items-center justify-center gap-x-4 gap-y-1 text-xs text-[var(--profile-faint)] sm:justify-start">
                  <span className="inline-flex items-center gap-1.5"><MapPin className="size-3.5" />在网络的一角</span>
                  <span>持续记录技术与普通日常</span>
                </div>
              </div>

              <div className="border-t border-[var(--profile-border)] pt-5 sm:border-l sm:border-t-0 sm:pl-8 sm:pt-0">
                <div className="grid grid-cols-3 gap-5 text-center sm:min-w-[280px]">
                  <ProfileStat value={stats?.articles} label="文章" />
                  <ProfileStat value={stats?.views} label="阅读" />
                  <ProfileStat value={stats?.likes} label="喜欢" />
                </div>
                <p className="mt-4 text-center font-mono text-[0.6rem] tracking-[0.14em] text-[var(--profile-faint)]">
                  KEEP RECORDING · KEEP CURIOUS
                </p>
              </div>
            </div>

            <div id="about-author" className="scroll-mt-24 pt-16 sm:pt-24">
              <ProfileSectionHeading eyebrow="the person behind the blog" title="关于作者" index="profile / 01" />

              <div className="mt-7 grid gap-5 lg:grid-cols-[1.35fr_0.85fr]">
                <div className="profile-paper-card rounded-[1.25rem] border border-[var(--profile-border)] bg-[var(--profile-card-soft)] p-6 sm:p-8">
                  <div className="flex items-start gap-4">
                    <span className="grid size-10 shrink-0 place-items-center rounded-2xl bg-[var(--profile-teal-wash)] text-[var(--profile-teal-deep)]">
                      <Compass className="size-5" />
                    </span>
                    <div>
                      <h3 className="font-playful text-xl font-bold text-[var(--profile-ink)] sm:text-2xl">你好，这里是睦子米。</h3>
                      <div className="mt-5 space-y-3 text-sm leading-8 text-[var(--profile-muted)] sm:text-base">
                        <p>我喜欢把复杂的问题拆开，把想法慢慢做成可以使用的东西。</p>
                        <p>这里记录前端与后端的实践，也记录设计、阅读和生活里的小发现。</p>
                        <p>不追求每天更新，只希望每次写下来的东西都值得回看。</p>
                      </div>
                    </div>
                  </div>
                </div>

                <div className="rounded-[1.25rem] border border-[var(--profile-teal)]/25 bg-[var(--profile-teal-wash)] p-6 sm:p-8">
                  <p className="font-mono text-[0.68rem] uppercase tracking-[0.22em] text-[var(--profile-teal-deep)]">now / focus</p>
                  <h3 className="mt-4 font-playful text-xl font-bold text-[var(--profile-ink)] sm:text-2xl">正在关注什么？</h3>
                  <div className="mt-5 flex flex-wrap gap-2">
                    {FOCUS_ITEMS.map((item) => (
                      <span key={item} className="rounded-full border border-white/80 bg-white/75 px-3 py-1.5 text-xs font-medium text-[var(--profile-teal-deep)] shadow-sm dark:border-white/10 dark:bg-white/10">
                        {item}
                      </span>
                    ))}
                  </div>
                  <p className="mt-6 text-sm leading-7 text-[var(--profile-muted)]">把技术做得清楚，把生活过得具体。</p>
                </div>
              </div>
            </div>

            <div className="pt-16 sm:pt-24">
              <ProfileSectionHeading eyebrow="what you can find here" title="这个空间记录什么" index="profile / 02" />
              <div className="mt-7 grid gap-4 md:grid-cols-3">
                {TOPIC_ITEMS.map((item) => {
                  const Icon = item.icon
                  const toneClasses = {
                    teal: "bg-[var(--profile-teal-wash)] text-[var(--profile-teal-deep)]",
                    sky: "bg-[var(--profile-sky-wash)] text-[var(--profile-sky-deep)]",
                    coral: "bg-[var(--profile-coral-wash)] text-[var(--profile-coral-deep)]",
                  } as const
                  return (
                    <article key={item.title} className="profile-topic-card group rounded-[1.15rem] border border-[var(--profile-border)] bg-[var(--profile-card)] p-6 shadow-[0_10px_30px_var(--profile-shadow-soft)] transition-transform duration-300 hover:-translate-y-1 sm:p-7">
                      <span className={`grid size-11 place-items-center rounded-2xl ${toneClasses[item.tone]}`}>
                        <Icon className="size-5" />
                      </span>
                      <h3 className="mt-6 font-playful text-xl font-bold text-[var(--profile-ink)]">{item.title}</h3>
                      <p className="mt-3 text-sm leading-7 text-[var(--profile-muted)]">{item.description}</p>
                      <span className="mt-7 inline-flex items-center gap-1.5 text-xs font-medium text-[var(--profile-faint)] transition-colors group-hover:text-[var(--profile-teal-deep)]">
                        explore the notes <ArrowUpRight className="size-3.5" />
                      </span>
                    </article>
                  )
                })}
              </div>
            </div>

            <div className="mt-16 flex flex-col gap-5 border-t border-[var(--profile-border)] py-8 sm:mt-24 sm:flex-row sm:items-center sm:justify-between">
              <div>
                <p className="font-mono text-[0.64rem] uppercase tracking-[0.2em] text-[var(--profile-faint)]">a small place for tech, design and ordinary days</p>
                <p className="mt-2 text-sm text-[var(--profile-muted)]">愿这里的每一次记录，都能在以后派上用场。</p>
              </div>
              <a href="#about-author" className="inline-flex items-center gap-1.5 text-sm font-medium text-[var(--profile-teal-deep)] hover:text-[var(--profile-sky-deep)]">
                回到作者介绍
                <ArrowUpRight className="size-4" />
              </a>
            </div>
          </Container>
        </section>
      </div>
    </SiteShell>
  )
}

function ProfileStat({ value, label }: { value: number | undefined; label: string }) {
  return (
    <div>
      <strong className="block font-playful text-2xl font-bold text-[var(--profile-navy)] sm:text-3xl">
        {value === undefined ? "—" : formatNumber(value)}
      </strong>
      <span className="mt-1 block text-xs tracking-[0.12em] text-[var(--profile-faint)]">{label}</span>
    </div>
  )
}

function ProfileSectionHeading({ eyebrow, title, index }: { eyebrow: string; title: string; index: string }) {
  return (
    <div className="flex items-end justify-between gap-4 border-b border-[var(--profile-border)] pb-5">
      <div>
        <p className="font-mono text-[0.68rem] uppercase tracking-[0.22em] text-[var(--profile-teal-deep)]">{eyebrow}</p>
        <h2 className="mt-3 font-playful text-3xl font-bold tracking-[-0.04em] text-[var(--profile-ink)] sm:text-4xl">{title}</h2>
      </div>
      <span className="hidden font-mono text-[0.64rem] tracking-[0.12em] text-[var(--profile-faint)] sm:block">{index}</span>
    </div>
  )
}
