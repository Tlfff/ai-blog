import Image from "next/image"
import { Container } from "@/components/layout/container"
import { SiteShell } from "@/components/layout/site-shell"
import { cn } from "@/lib/utils"

type UtilityVariant = "history" | "notifications"

const VARIANT_CONFIG = {
  history: {
    image: "/kv/bq-1.png",
    imagePosition: "object-[center_28%]",
    overlay: "bg-[linear-gradient(90deg,rgba(12,68,103,0.82),rgba(30,132,166,0.52),rgba(44,157,156,0.3))]",
  },
  notifications: {
    image: "/kv/bq-2.png",
    imagePosition: "object-[center_42%]",
    overlay: "bg-[linear-gradient(90deg,rgba(31,63,105,0.88),rgba(65,103,158,0.64),rgba(104,91,157,0.52))]",
  },
} as const

export function UtilityCenterShell({
  variant,
  eyebrow,
  title,
  description,
  metricLabel,
  metricValue,
  children,
}: {
  variant: UtilityVariant
  eyebrow: string
  title: string
  description: string
  metricLabel: string
  metricValue: string | number
  children: React.ReactNode
}) {
  const config = VARIANT_CONFIG[variant]

  return (
    <SiteShell immersiveHeader>
      <div className={cn("utility-center-page min-h-screen overflow-hidden bg-[var(--utility-background)] text-[var(--utility-ink)]", `utility-${variant}`)}>
        <section className="relative isolate min-h-[330px] overflow-hidden text-white sm:min-h-[390px]">
          <Image src={config.image} alt="" fill priority sizes="100vw" className={cn("object-cover", config.imagePosition)} />
          <div className={cn("absolute inset-0", config.overlay)} aria-hidden />
          <div className="absolute inset-x-0 bottom-0 h-40 bg-gradient-to-t from-[#173d5d]/55 to-transparent" aria-hidden />

          <Container className="relative flex min-h-[330px] items-center pb-16 pt-24 sm:min-h-[390px] sm:pb-24">
            <div className="flex w-full flex-col gap-7 sm:flex-row sm:items-center sm:justify-between">
              <div className="max-w-4xl">
                <p className="font-mono text-[0.66rem] font-semibold uppercase tracking-[0.27em] text-white/78">{eyebrow}</p>
                <h1 className="mt-4 font-playful text-[clamp(2.5rem,5vw,4.4rem)] font-bold leading-[1.05] tracking-[-0.045em] [text-shadow:0_3px_18px_rgba(0,25,48,0.28)]">{title}</h1>
                <p className="mt-5 text-sm leading-7 text-white/86 sm:text-base">{description}</p>
              </div>
              <div className="w-fit min-w-44 rounded-2xl border border-white/25 bg-[#173b5c]/25 px-5 py-4 backdrop-blur-sm">
                <p className="font-mono text-[0.58rem] uppercase tracking-[0.18em] text-white/68">{metricLabel}</p>
                <p className="mt-1 font-playful text-3xl font-bold">{metricValue}</p>
              </div>
            </div>
          </Container>

          <div className="pointer-events-none absolute inset-x-0 bottom-0 z-10 h-20 sm:h-28" aria-hidden>
            <svg viewBox="0 0 1440 112" preserveAspectRatio="none" className="h-full w-full">
              <path d="M0 48C210 82 398 77 590 61C815 42 994 27 1168 51C1285 67 1366 70 1440 60V112H0Z" fill="var(--utility-wave)" />
              <path d="M0 72C188 58 353 99 588 87C818 75 1002 47 1190 70C1290 82 1365 88 1440 80V112H0Z" fill="var(--utility-background)" />
            </svg>
          </div>
        </section>

        <Container className="relative z-20 -mt-9 max-w-[1200px] pb-16 sm:-mt-14 sm:pb-24">
          {children}
        </Container>
      </div>
    </SiteShell>
  )
}

export function UtilityPanel({ children, className }: { children: React.ReactNode; className?: string }) {
  return <section className={cn("rounded-[1.25rem] border border-[var(--utility-border)] bg-[var(--utility-card)] shadow-[0_16px_46px_var(--utility-shadow)]", className)}>{children}</section>
}
