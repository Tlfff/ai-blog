"use client"

import { useEffect, useState } from "react"
import Image from "next/image"
import { AnimatePresence, motion, useReducedMotion } from "motion/react"
import { ChevronLeft, ChevronRight } from "lucide-react"
import { cn } from "@/lib/utils"

const AUTOPLAY_DELAY = 6500
const SWIPE_THRESHOLD = 48

const HERO_SLIDES = [
  {
    src: "/kv/bocchi-sky-wide.png",
    alt: "蓝天下的乐队成员",
    objectPosition: "center 48%",
  },
  {
    src: "/kv/bocchi-peace.jpg",
    alt: "戴着墨镜比出胜利手势的少女",
    objectPosition: "center 32%",
  },
  {
    src: "/kv/bocchi-earphone.jpg",
    alt: "戴着耳机的粉发少女",
    objectPosition: "center 30%",
  },
] as const

export function HeroVisualCarousel() {
  const [activeIndex, setActiveIndex] = useState(0)
  const [paused, setPaused] = useState(false)
  const [touchStart, setTouchStart] = useState<number | null>(null)
  const reduceMotion = useReducedMotion()

  function showSlide(index: number) {
    setActiveIndex((index + HERO_SLIDES.length) % HERO_SLIDES.length)
  }

  useEffect(() => {
    if (paused || reduceMotion) return

    const timer = window.setTimeout(() => {
      setActiveIndex((currentIndex) => (currentIndex + 1) % HERO_SLIDES.length)
    }, AUTOPLAY_DELAY)
    return () => window.clearTimeout(timer)
  }, [activeIndex, paused, reduceMotion])

  const activeSlide = HERO_SLIDES[activeIndex]

  return (
    <div
      className="absolute inset-0"
      role="region"
      aria-roledescription="轮播图"
      aria-label="首页主视觉"
      tabIndex={0}
      onMouseEnter={() => setPaused(true)}
      onMouseLeave={() => setPaused(false)}
      onFocusCapture={() => setPaused(true)}
      onBlurCapture={(event) => {
        if (!event.currentTarget.contains(event.relatedTarget)) setPaused(false)
      }}
      onKeyDown={(event) => {
        if (event.key === "ArrowLeft") showSlide(activeIndex - 1)
        if (event.key === "ArrowRight") showSlide(activeIndex + 1)
      }}
      onTouchStart={(event) => setTouchStart(event.touches[0]?.clientX ?? null)}
      onTouchEnd={(event) => {
        if (touchStart === null) return
        const delta = (event.changedTouches[0]?.clientX ?? touchStart) - touchStart
        if (Math.abs(delta) >= SWIPE_THRESHOLD) {
          showSlide(activeIndex + (delta < 0 ? 1 : -1))
        }
        setTouchStart(null)
      }}
    >
      <AnimatePresence initial={false} mode="popLayout">
        <motion.div
          key={activeSlide.src}
          className="absolute inset-0"
          initial={reduceMotion ? false : { opacity: 0, scale: 1.035 }}
          animate={{ opacity: 1, scale: 1 }}
          exit={{ opacity: 0 }}
          transition={{ duration: reduceMotion ? 0 : 0.8, ease: [0.16, 1, 0.3, 1] }}
        >
          <Image
            src={activeSlide.src}
            alt={activeSlide.alt}
            fill
            priority={activeIndex === 0}
            sizes="100vw"
            className="object-cover"
            style={{ objectPosition: activeSlide.objectPosition }}
          />
        </motion.div>
      </AnimatePresence>

      <div
        className="absolute inset-0 bg-[linear-gradient(180deg,rgba(9,12,18,0.42)_0%,rgba(9,12,18,0.18)_48%,rgba(14,14,19,0.48)_100%)]"
        aria-hidden
      />

      <div className="absolute bottom-24 right-5 z-30 flex items-center gap-3 sm:bottom-28 sm:right-8 lg:right-12">
        <div className="flex items-center gap-2" role="tablist" aria-label="选择主视觉">
          {HERO_SLIDES.map((slide, index) => (
            <button
              key={slide.src}
              type="button"
              role="tab"
              aria-selected={index === activeIndex}
              aria-label={`显示第 ${index + 1} 张主视觉`}
              onClick={() => showSlide(index)}
              className="group grid h-8 place-items-center px-1"
            >
              <span
                className={cn(
                  "block h-1.5 rounded-full bg-white/55 transition-all duration-300 group-hover:bg-white",
                  index === activeIndex ? "w-8 bg-[#f39ab3]" : "w-3",
                )}
              />
            </button>
          ))}
        </div>
        <div className="hidden overflow-hidden rounded-md border border-white/30 bg-black/20 backdrop-blur-sm sm:flex">
          <button
            type="button"
            onClick={() => showSlide(activeIndex - 1)}
            aria-label="上一张主视觉"
            className="grid size-9 place-items-center text-white transition-colors hover:bg-white/15"
          >
            <ChevronLeft className="size-4" />
          </button>
          <button
            type="button"
            onClick={() => showSlide(activeIndex + 1)}
            aria-label="下一张主视觉"
            className="grid size-9 place-items-center border-l border-white/25 text-white transition-colors hover:bg-white/15"
          >
            <ChevronRight className="size-4" />
          </button>
        </div>
      </div>
    </div>
  )
}
