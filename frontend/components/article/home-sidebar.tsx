"use client"

import Image from "next/image"
import useSWR from "swr"
import { getHotArticles, getStats } from "@/api/articles"
import { formatNumber } from "@/lib/format"
import { RankingList } from "./ranking-list"

export function HomeSidebar() {
  const { data: stats } = useSWR("home-stats", getStats)
  const { data: hotArticles = [], isLoading: hotLoading, error: hotError } = useSWR(
    "home-hot-articles",
    () => getHotArticles(5),
    { errorRetryCount: 1 },
  )

  return (
    <div className="home-sider mx-auto max-w-sm lg:sticky lg:top-0 lg:max-w-none">
      <section className="home-author-card rounded-[1.25rem] border border-[var(--home-sidebar-border)] bg-[var(--home-sidebar-card)] p-5 shadow-[0_12px_32px_var(--home-sidebar-shadow)] lg:rounded-none lg:border-0 lg:bg-transparent lg:p-0 lg:shadow-none">
        <div className="flex items-center gap-4 text-left lg:flex-col lg:text-center">
          <div className="relative size-24 shrink-0 overflow-hidden rounded-full border-2 border-[var(--home-avatar-ring)] bg-[var(--home-image-surface)] shadow-[0_0_0_6px_var(--home-avatar-halo)] lg:size-32">
            <Image src="/kv/bocchi-sunglasses.jpg" alt="睦子米" fill sizes="128px" className="object-cover" />
          </div>

          <div className="min-w-0 lg:mt-1">
            <h2 className="font-playful text-2xl font-bold tracking-wide text-[var(--home-text)] lg:text-3xl">睦子米</h2>
            <p className="mt-2 text-xs leading-6 text-[var(--home-muted)] lg:mt-4 lg:text-sm lg:leading-7">热爱写代码，也热爱生活。<br />在技术与日常之间寻找灵感。</p>
          </div>
        </div>

        <div className="my-5 h-px bg-[var(--home-divider)] lg:my-7" aria-hidden />
        <dl className="grid grid-cols-3 gap-2 text-center">
          <Stat value={stats?.articles} label="文章" tone="coral" />
          <Stat value={stats?.views} label="阅读" tone="sky" />
          <Stat value={stats?.likes} label="喜欢" tone="pink" />
        </dl>
      </section>

      <div className="home-ranking-card mt-5 rounded-[1.25rem] border border-[var(--home-sidebar-border)] bg-[var(--home-sidebar-card)] px-5 pb-4 shadow-[0_12px_32px_var(--home-sidebar-shadow)] lg:mt-0 lg:rounded-none lg:border-0 lg:bg-transparent lg:px-0 lg:pb-0 lg:shadow-none">
        <RankingList articles={hotArticles} loading={hotLoading} failed={Boolean(hotError)} />
      </div>
    </div>
  )
}

function Stat({ value, label, tone }: { value: number | undefined; label: string; tone: "coral" | "sky" | "pink" }) {
  const toneClass = { coral: "text-[var(--home-accent)]", sky: "text-[var(--home-hot-sky)]", pink: "text-[var(--home-hot-coral)]" }[tone]
  return <div><dd className={`font-playful text-2xl font-bold ${toneClass}`}>{value === undefined ? "—" : formatNumber(value)}</dd><dt className="mt-1 text-xs tracking-[0.12em] text-[var(--home-muted)]">{label}</dt></div>
}
