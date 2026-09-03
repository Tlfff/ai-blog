"use client"

import { Moon, Sun } from "lucide-react"
import { cn } from "@/lib/utils"

const THEME_STORAGE_KEY = "site-theme"

export function ThemeToggle({ inverted = false }: { inverted?: boolean }) {
  function toggleTheme() {
    const root = document.documentElement
    const nextDark = !root.classList.contains("dark")
    root.classList.toggle("dark", nextDark)
    root.dataset.theme = nextDark ? "dark" : "light"
    localStorage.setItem(THEME_STORAGE_KEY, nextDark ? "dark" : "light")
  }

  return (
    <button
      type="button"
      onClick={toggleTheme}
      aria-label="切换日间或深夜模式"
      className={cn(
        "grid size-9 place-items-center rounded-md transition-colors",
        inverted
          ? "text-white hover:bg-white/15"
          : "text-ink-soft hover:bg-sakura-wash hover:text-sakura-deep",
      )}
    >
      <Moon className="size-5 dark:hidden" />
      <Sun className="hidden size-5 dark:block" />
    </button>
  )
}
