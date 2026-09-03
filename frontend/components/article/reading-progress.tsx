"use client"

import { useEffect, useState } from "react"

export function ReadingProgress() {
  const [progress, setProgress] = useState(0)

  useEffect(() => {
    const updateProgress = () => {
      const scrollable = document.documentElement.scrollHeight - window.innerHeight
      setProgress(scrollable > 0 ? Math.min(100, (window.scrollY / scrollable) * 100) : 0)
    }

    updateProgress()
    window.addEventListener("scroll", updateProgress, { passive: true })
    window.addEventListener("resize", updateProgress)
    return () => {
      window.removeEventListener("scroll", updateProgress)
      window.removeEventListener("resize", updateProgress)
    }
  }, [])

  return (
    <div className="fixed right-3 top-1/2 z-40 hidden h-40 -translate-y-1/2 items-end gap-2 xl:flex" aria-label={`阅读进度 ${Math.round(progress)}%`}>
      <span className="label-meta [writing-mode:vertical-rl] text-[var(--article-faint)]">阅读进度</span>
      <div className="relative h-full w-1 rounded-full bg-[var(--article-divider)]">
        <span
          className="absolute inset-x-0 top-0 rounded-full bg-[var(--article-accent)] transition-[height] duration-150"
          style={{ height: `${progress}%` }}
        />
      </div>
    </div>
  )
}
