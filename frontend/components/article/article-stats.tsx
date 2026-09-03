import Link from "next/link"
import { Eye, MessageSquare, ThumbsUp } from "lucide-react"
import { formatNumber } from "@/lib/format"
import { cn } from "@/lib/utils"

interface ArticleStatsProps {
  views: number
  likes: number
  comments: number
  className?: string
  articleId?: string
}

export function ArticleStats({ views, likes, comments, className, articleId }: ArticleStatsProps) {
  return (
    <div className={cn("flex items-center gap-4 text-xs text-muted-foreground", className)}>
      <span className="inline-flex items-center gap-1.5 group">
        <div className="p-1 rounded-lg bg-primary/5 group-hover:bg-primary/10 transition-colors duration-200">
          <Eye className="size-3.5 text-primary/70 group-hover:text-primary transition-colors" />
        </div>
        <span className="group-hover:text-primary/80 transition-colors">{formatNumber(views)}</span>
      </span>
      <span className="inline-flex items-center gap-1.5 group">
        <div className="p-1 rounded-lg bg-primary/5 group-hover:bg-primary/10 transition-colors duration-200">
          <ThumbsUp className="size-3.5 text-primary/70 group-hover:text-primary transition-colors" />
        </div>
        <span className="group-hover:text-primary/80 transition-colors">{formatNumber(likes)}</span>
      </span>
      <Link
        href={`/articles/${articleId}#comments`}
        className="inline-flex items-center gap-1.5 group hover:text-primary transition-colors duration-200"
      >
        <div className="p-1 rounded-lg bg-primary/5 group-hover:bg-primary/10 transition-colors duration-200">
          <MessageSquare className="size-3.5 text-primary/70 group-hover:text-primary transition-colors" />
        </div>
        <span>{formatNumber(comments)}</span>
      </Link>
    </div>
  )
}