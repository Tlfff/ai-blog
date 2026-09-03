"use client"

import { useRouter } from "next/navigation"
import { useState } from "react"
import { Search } from "lucide-react"
import { cn } from "@/lib/utils"

export function SearchBox({ className, defaultValue = "" }: { className?: string; defaultValue?: string }) {
  const router = useRouter()
  const [value, setValue] = useState(defaultValue)

  function submit(e: React.FormEvent) {
    e.preventDefault()
    const kw = value.trim()
    if (kw) router.push(`/search?q=${encodeURIComponent(kw)}`)
  }

  return (
    <form onSubmit={submit} className={cn("relative", className)} role="search">
      <Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
      <input
        type="search"
        value={value}
        onChange={(e) => setValue(e.target.value)}
        placeholder="搜索标题、正文或标签..."
        aria-label="搜索"
        className="h-9 w-full rounded-lg border border-border bg-muted/50 pl-9 pr-3 text-sm outline-none transition-colors placeholder:text-muted-foreground focus:border-ring focus:bg-background focus:ring-2 focus:ring-ring/40"
      />
    </form>
  )
}
