"use client"

import Image from "next/image"
import useSWR from "swr"
import { getStats } from "@/api/articles"
import { formatNumber } from "@/lib/format"

export function HomeSidebar() {
  const { data: stats } = useSWR("home-stats", getStats)

  return (
    <div className="home-sider mx-auto flex max-w-sm flex-col items-center text-center lg:sticky lg:top-0 lg:max-w-none lg:items-stretch">
      <div className="relative mx-auto size-32 overflow-hidden rounded-full border-2 border-[var(--home-avatar-ring)] bg-[var(--home-image-surface)] shadow-[0_0_0_6px_var(--home-avatar-halo)]">
        <Image
          src="/kv/bocchi-sunglasses.jpg"
          alt="睦子米"
          fill
          sizes="128px"
          className="object-cover"
        />
      </div>

      <h2 className="font-playful mt-5 text-3xl font-bold tracking-wide text-[var(--home-text)]">
        睦子米
      </h2>
      <p className="mt-4 text-sm leading-7 text-[var(--home-muted)]">
        热爱写代码，也热爱生活。
        <br />
        在技术与日常之间寻找灵感。
      </p>

      <div className="my-7 h-px bg-[var(--home-divider)]" aria-hidden />
      <dl className="grid grid-cols-3 gap-2">
        <Stat value={stats?.articles} label="文章" />
        <Stat value={stats?.views} label="阅读" />
        <Stat value={stats?.likes} label="喜欢" />
      </dl>

    </div>
  )
}

function Stat({ value, label }: { value: number | undefined; label: string }) {
  return (
    <div>
      <dd className="font-playful text-2xl font-bold text-[var(--home-accent)]">
        {value === undefined ? "—" : formatNumber(value)}
      </dd>
      <dt className="mt-1 text-xs tracking-[0.12em] text-[var(--home-muted)]">{label}</dt>
    </div>
  )
}
