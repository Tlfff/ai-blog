import Link from "next/link"
import { Hash } from "lucide-react"
import { cn } from "@/lib/utils"
import type { Tag } from "@/types"

export function TagCloud({ tags, activeTag }: { tags: Tag[]; activeTag?: string }) {
  return (
    <div className="p-4 card-paper rounded-md">
      <h3 className="mb-3 flex items-center gap-2 text-sm font-semibold">
        <div className="p-1 rounded-lg bg-primary/10">
          <Hash className="size-4 text-primary" />
        </div>
        热门标签
      </h3>
      <div className="flex flex-wrap gap-2">
        <Link
          href="/"
          className={cn(
            "rounded-full px-3 py-1 text-xs font-medium transition-all duration-200",
            !activeTag
              ? "bg-sakura-deep text-primary-foreground"
              : "bg-card/50 text-muted-foreground hover:bg-primary/10 hover:text-primary",
          )}
        >
          全部
        </Link>
        {tags.map((tag) => {
          const active = tag.name === activeTag
          return (
            <Link
              key={tag.id}
              href={`/search?q=${encodeURIComponent(tag.name)}&page=1`}
              className={cn(
                "rounded-full px-3 py-1 text-xs font-medium transition-all duration-200",
                active
                  ? "bg-sakura-deep text-primary-foreground"
                  : "bg-card/50 text-muted-foreground hover:bg-primary/10 hover:text-primary",
              )}
            >
              {tag.name}
              <span className={cn("ml-1", active ? "opacity-80" : "opacity-60")}>{tag.count}</span>
            </Link>
          )
        })}
      </div>
    </div>
  )
}
