"use client"

import { useEffect, useState } from "react"
import { ListTree } from "lucide-react"
import { cn } from "@/lib/utils"

export interface ArticleHeading {
  id: string
  text: string
  level: 1 | 2 | 3
}

export function extractArticleHeadings(content: string): ArticleHeading[] {
  const counts = new Map<string, number>()
  const headings: ArticleHeading[] = []

  for (const line of content.split("\n")) {
    const match = /^(#{1,3})\s+(.+?)\s*$/.exec(line)
    if (!match) continue

    const level = match[1].length as 1 | 2 | 3
    const text = match[2].replace(/[*_`~]/g, "").trim()
    if (!text) continue

    const baseId = slugifyHeading(text)
    const count = counts.get(baseId) ?? 0
    counts.set(baseId, count + 1)
    headings.push({ id: count === 0 ? baseId : `${baseId}-${count + 1}`, text, level })
  }

  return headings
}

export function slugifyHeading(text: string): string {
  const slug = text
    .toLowerCase()
    .trim()
    .replace(/[^\p{L}\p{N}\s-]/gu, "")
    .replace(/[\s-]+/g, "-")
  return slug || "section"
}

export function ArticleToc({ headings }: { headings: ArticleHeading[] }) {
  const [activeId, setActiveId] = useState(headings[0]?.id ?? "")

  useEffect(() => {
    if (headings.length === 0) return

    const elements = headings
      .map((heading) => document.getElementById(heading.id))
      .filter((element): element is HTMLElement => Boolean(element))
    if (elements.length === 0) return

    const getHeaderOffset = () => {
      const header = document.querySelector("header")
      return (header instanceof HTMLElement ? header.getBoundingClientRect().height : 56) + 24
    }

    const updateActiveHeading = () => {
      const threshold = getHeaderOffset() + 16
      let current = elements[0]
      for (const element of elements) {
        if (element.getBoundingClientRect().top <= threshold) current = element
        else break
      }
      setActiveId(current.id)
    }

    updateActiveHeading()
    window.addEventListener("scroll", updateActiveHeading, { passive: true })
    window.addEventListener("resize", updateActiveHeading)

    return () => {
      window.removeEventListener("scroll", updateActiveHeading)
      window.removeEventListener("resize", updateActiveHeading)
    }
  }, [headings])

  function handleHeadingClick(event: React.MouseEvent<HTMLAnchorElement>, id: string) {
    event.preventDefault()
    const target = document.getElementById(id)
    if (!target) return

    const header = document.querySelector("header")
    const headerHeight = header instanceof HTMLElement ? header.getBoundingClientRect().height : 56
    const targetTop = window.scrollY + target.getBoundingClientRect().top - headerHeight - 24
    window.history.pushState(null, "", `#${encodeURIComponent(id)}`)
    setActiveId(id)
    window.scrollTo({ top: Math.max(0, targetTop), behavior: "smooth" })
  }

  if (headings.length === 0) {
    return (
      <div className="flex items-center gap-2 text-sm text-[var(--article-muted)]">
        <ListTree className="size-4" />
        <span>正文阅读</span>
      </div>
    )
  }

  return (
    <nav aria-label="文章目录">
      <div className="mb-4 flex items-center gap-2 text-sm font-semibold text-[var(--article-text)]">
        <ListTree className="size-4 text-[var(--article-accent)]" />
        <span>目录</span>
      </div>
      <ol className="space-y-1 border-l border-[var(--article-divider)]">
        {headings.map((heading) => (
          <li key={heading.id}>
            <a
              href={`#${heading.id}`}
              onClick={(event) => handleHeadingClick(event, heading.id)}
              className={cn(
                "relative block py-2 text-sm leading-6 transition-colors",
                heading.level === 1 && "pl-4",
                heading.level === 2 && "pl-7",
                heading.level === 3 && "pl-10 text-xs",
                activeId === heading.id
                  ? "font-medium text-[var(--article-accent)] before:absolute before:-left-px before:inset-y-1 before:w-0.5 before:bg-[var(--article-accent)]"
                  : "text-[var(--article-muted)] hover:text-[var(--article-accent)]",
              )}
            >
              {heading.text}
            </a>
          </li>
        ))}
      </ol>
    </nav>
  )
}
