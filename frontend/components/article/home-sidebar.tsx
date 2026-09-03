"use client"

import Image from "next/image"
import useSWR from "swr"
import { getStats } from "@/api/articles"
import { formatNumber } from "@/lib/format"

export function HomeSidebar() {
  const { data: stats } = useSWR("home-stats", getStats)

  return (
    <div className="mx-auto max-w-sm text-center lg:max-w-none">
      <div className="relative mx-auto size-28 overflow-hidden rounded-full border-2 border-[#f2c9d4]/80 bg-white/5 shadow-[0_0_0_6px_rgba(232,77,122,0.08)] sm:size-32">
        <Image
          src="/kv/bocchi-sunglasses.jpg"
          alt="睦子米"
          fill
          sizes="128px"
          className="object-cover"
        />
      </div>

      <h2 className="font-playful mt-5 text-3xl font-bold tracking-wide text-[#fffaf3]">
        睦子米
      </h2>
      <p className="mx-auto mt-4 max-w-[15rem] text-sm leading-7 text-white/62">
        热爱写代码，也热爱生活。
        <br />
        在技术与日常之间寻找灵感。
      </p>

      <div className="mt-7 h-px bg-white/14" aria-hidden />
      <dl className="mt-7 grid grid-cols-3 gap-2">
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
      <dd className="font-playful text-2xl font-bold text-[#f46f98]">
        {value === undefined ? "—" : formatNumber(value)}
      </dd>
      <dt className="mt-1 text-xs tracking-[0.12em] text-white/58">{label}</dt>
    </div>
  )
}
