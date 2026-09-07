"use client"

import { useEffect, useRef, useState } from "react"
import { motion, useReducedMotion, useScroll, useTransform } from "motion/react"
import {
  SITE_INTRO_COMPLETE_DATASET_KEY,
  SITE_INTRO_COMPLETE_EVENT,
} from "@/components/layout/site-intro"
import type { HeroSlide } from "@/lib/site-images"
import { HeroVisualCarousel } from "./hero-visual-carousel"

export function HomeHero({ slides }: { slides: HeroSlide[] }) {
  const [introComplete, setIntroComplete] = useState(false)
  const sectionRef = useRef<HTMLElement>(null)
  const reduceMotion = useReducedMotion()
  const heroVisible = introComplete || Boolean(reduceMotion)
  const { scrollYProgress } = useScroll({
    target: sectionRef,
    offset: ["start start", "end end"],
  })
  const copyY = useTransform(scrollYProgress, [0, 1], ["0%", "-16%"])
  const copyOpacity = useTransform(scrollYProgress, [0, 0.72, 1], [1, 0.86, 0.18])
  const imageScale = useTransform(scrollYProgress, [0, 1], [1, 1.035])
  const waveY = useTransform(scrollYProgress, [0, 1], ["105%", "0%"])
  const waveOpacity = useTransform(scrollYProgress, [0, 0.12, 1], [0, 1, 1])

  useEffect(() => {
    const revealHero = () => setIntroComplete(true)

    if (document.documentElement.dataset[SITE_INTRO_COMPLETE_DATASET_KEY] === "true") {
      revealHero()
    }

    window.addEventListener(SITE_INTRO_COMPLETE_EVENT, revealHero)
    return () => window.removeEventListener(SITE_INTRO_COMPLETE_EVENT, revealHero)
  }, [])

  return (
    <section ref={sectionRef} data-home-hero className="relative h-[125svh] bg-[#17171d]">
      <div className="sticky top-0 h-[100svh] overflow-hidden bg-[#17171d]">
        <motion.div style={{ scale: reduceMotion ? 1 : imageScale }} className="absolute inset-0">
          <HeroVisualCarousel slides={slides} />
        </motion.div>

        <motion.div
          style={{ y: reduceMotion ? 0 : copyY, opacity: reduceMotion ? 1 : copyOpacity }}
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

        <motion.div
          data-home-wave
          style={{ y: waveY, opacity: waveOpacity }}
          className="pointer-events-none absolute inset-x-0 bottom-0 z-20 h-24 sm:h-32"
          aria-hidden
        >
          <svg
            viewBox="0 0 1440 128"
            preserveAspectRatio="none"
            className="h-full w-full"
            role="presentation"
          >
            <path
              d="M0 56C225 92 390 94 584 74C798 52 923 42 1115 58C1249 69 1345 72 1440 58V128H0Z"
              fill="var(--home-wave)"
            />
            <path
              d="M0 76C211 62 342 112 590 99C816 87 995 54 1191 78C1287 90 1363 98 1440 89V128H0Z"
              fill="var(--home-surface)"
            />
          </svg>
        </motion.div>
      </div>
    </section>
  )
}
