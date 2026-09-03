import Link from "next/link"
import { Code2 } from "lucide-react"
import { Container } from "./container"

export function SiteFooter() {
  return (
    <footer className="mt-16 border-t border-border bg-muted/30">
      <Container className="flex flex-col gap-6 py-10 sm:flex-row sm:items-center sm:justify-between">
        <div className="flex items-center gap-2">
          <span className="flex size-7 items-center justify-center rounded-lg bg-primary text-primary-foreground">
            <Code2 className="size-4" />
          </span>
          <div>
            <p className="text-sm font-semibold">Link start！</p>
            <p className="text-xs text-muted-foreground">睦子米的个人博客</p>
          </div>
        </div>
        <nav className="flex flex-wrap gap-x-6 gap-y-2 text-sm text-muted-foreground">
          <Link href="/" className="hover:text-foreground">
            首页
          </Link>
          <Link href="/search" className="hover:text-foreground">
            搜索
          </Link>
          <Link href="/profile" className="hover:text-foreground">
            个人中心
          </Link>
          <Link href="/admin" className="hover:text-foreground">
            管理后台
          </Link>
        </nav>
        <p className="text-xs text-muted-foreground">© {new Date().getFullYear()} Link start！</p>
      </Container>
    </footer>
  )
}
