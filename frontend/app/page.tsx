"use client"

import { use, useEffect, useRef, useState } from "react"
import { motion, useReducedMotion, useScroll, useTransform } from "motion/react"
import { ArticleList } from "@/components/article/article-list"
import { HomeSidebar } from "@/components/article/home-sidebar"
import { HeroVisualCarousel } from "@/components/home/hero-visual-carousel"
import { Container } from "@/components/layout/container"
import {
  SITE_INTRO_COMPLETE_DATASET_KEY,
  SITE_INTRO_COMPLETE_EVENT,
} from "@/components/layout/site-intro"
import { SiteShell } from "@/components/layout/site-shell"

function HeroSection() {
  const [introComplete, setIntroComplete] = useState(false)
  const sectionRef = useRef<HTMLElement>(null)
  const reduceMotion = useReducedMotion()
  const heroVisible = introComplete || Boolean(reduceMotion)
  const { scrollYProgress } = useScroll({
    target: sectionRef,
    offset: ["start start", "end start"],
  })
  const copyY = useTransform(scrollYProgress, [0, 1], ["0%", "-18%"])

  useEffect(() => {
    const revealHero = () => setIntroComplete(true)

    if (document.documentElement.dataset[SITE_INTRO_COMPLETE_DATASET_KEY] === "true") {
      revealHero()
    }

    window.addEventListener(SITE_INTRO_COMPLETE_EVENT, revealHero)
    return () => window.removeEventListener(SITE_INTRO_COMPLETE_EVENT, revealHero)
  }, [])

  return (
    <section
      ref={sectionRef}
      className="relative h-[64svh] min-h-[560px] max-h-[760px] overflow-hidden bg-[#17171d] sm:h-[62svh]"
    >
      <HeroVisualCarousel />

      <motion.div
        style={{ y: reduceMotion ? 0 : copyY }}
        className="pointer-events-none relative z-10 flex h-full items-center justify-center px-5 pb-16 pt-14 text-center sm:pb-20"
      >
        <div>
          <motion.h1
            className="font-playful text-[clamp(2.65rem,7vw,5.5rem)] font-bold leading-[1.05] tracking-[-0.035em] text-[#fffaf3] [text-shadow:0_3px_18px_rgba(0,0,0,0.42)]"
            initial={{ opacity: 0, y: 18 }}
            animate={heroVisible ? { opacity: 1, y: 0 } : { opacity: 0, y: 18 }}
            transition={{ duration: 0.72, ease: [0.16, 1, 0.3, 1] }}
          >
            睦子米的个人博客
          </motion.h1>
          <motion.p
            className="mx-auto mt-5 max-w-2xl text-sm font-medium tracking-[0.08em] text-[#fffaf3]/90 [text-shadow:0_2px_12px_rgba(0,0,0,0.55)] sm:text-lg"
            initial={{ opacity: 0, y: 12 }}
            animate={heroVisible ? { opacity: 1, y: 0 } : { opacity: 0, y: 12 }}
            transition={{ delay: 0.12, duration: 0.55 }}
          >
            记录技术、生活与正在发生的灵感。
          </motion.p>
        </div>
      </motion.div>

      <div className="pointer-events-none absolute inset-x-0 bottom-0 z-20 h-24 sm:h-32" aria-hidden>
        <svg
          viewBox="0 0 1440 128"
          preserveAspectRatio="none"
          className="h-full w-full"
          role="presentation"
        >
          <path
            d="M0 56C225 92 390 94 584 74C798 52 923 42 1115 58C1249 69 1345 72 1440 58V128H0Z"
            fill="#f2c9d4"
          />
          <path
            d="M0 76C211 62 342 112 590 99C816 87 995 54 1191 78C1287 90 1363 98 1440 89V128H0Z"
            fill="#17171d"
          />
        </svg>
      </div>
    </section>
  )
}

function SectionHeading({ children }: { children: React.ReactNode }) {
  return (
    <div className="mb-8 flex items-center gap-4 sm:mb-10 sm:gap-6">
      <span className="h-px flex-1 bg-white/20" aria-hidden />
      <h2 className="font-playful shrink-0 text-2xl font-bold tracking-wide text-[#fffaf3] sm:text-3xl">
        {children}
      </h2>
      <span className="h-px flex-1 bg-white/20" aria-hidden />
    </div>
  )
}

export default function HomePage({
  searchParams,
}: {
  searchParams: Promise<{ tag?: string; tab?: string }>
}) {
  const { tag } = use(searchParams)

  return (
    <SiteShell immersiveHeader>
      <HeroSection />
      <section className="relative bg-[#17171d] text-[#fffaf3]">
        <Container className="max-w-[1240px] pb-16 pt-7 sm:pb-20 sm:pt-10 lg:pb-24">
          <div className="grid gap-12 lg:grid-cols-[240px_minmax(0,1fr)] lg:gap-14 xl:gap-20">
            <aside id="about" className="scroll-mt-24 lg:sticky lg:top-24 lg:self-start">
              <HomeSidebar />
            </aside>

            <main id="latest" className="min-w-0 scroll-mt-24">
              <SectionHeading>{tag ? `# ${tag}` : "最新文章"}</SectionHeading>
              <ArticleList tag={tag} />
            </main>
          </div>

          <footer className="mt-16 flex items-center gap-5 border-t border-white/10 pt-6 text-xs text-white/45">
            <span>© 2026 睦子米</span>
            <span className="h-px flex-1 bg-white/10" aria-hidden />
            <span className="font-playful text-sm text-white/55">Link start！</span>
          </footer>
        </Container>
      </section>
    </SiteShell>
  )
}
