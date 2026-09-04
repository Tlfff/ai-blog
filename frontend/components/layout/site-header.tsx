"use client"

import { useEffect, useState } from "react"
import Link from "next/link"
import { Code2, PenSquare, Search } from "lucide-react"
import { NotificationBell } from "@/components/notification/notification-bell"
import { ThemeToggle } from "@/components/theme/theme-toggle"
import { buttonVariants } from "@/components/ui/button"
import { useAuth } from "@/hooks/use-auth"
import { cn } from "@/lib/utils"
import { Container } from "./container"
import { SearchBox } from "./search-box"
import { UserMenu } from "./user-menu"

export function SiteHeader({ overlay = false }: { overlay?: boolean }) {
  const { isLoggedIn, isAdmin } = useAuth()
  const [scrolled, setScrolled] = useState(false)
  const transparent = overlay && !scrolled

  useEffect(() => {
    const handleScroll = () => setScrolled(window.scrollY > 36)
    handleScroll()
    window.addEventListener("scroll", handleScroll, { passive: true })
    return () => window.removeEventListener("scroll", handleScroll)
  }, [])

  const navTextClass = transparent
    ? "text-white/90 hover:text-white"
    : "text-ink-soft hover:text-ink"

  return (
    <header
      className={cn(
        "fixed inset-x-0 top-0 z-50 transition-all duration-300 ease-out",
        transparent
          ? "h-16 bg-transparent"
          : "h-14 border-b border-border bg-paper/95 shadow-[0_1px_0_rgba(27,25,32,0.03)] backdrop-blur-md",
      )}
    >
      <Container className="flex h-full max-w-[1380px] items-center gap-3 sm:gap-5">
        <Link href="/" className="group flex shrink-0 items-center gap-2.5">
          <span
            className={cn(
              "flex size-8 items-center justify-center rounded-[4px] border transition-colors duration-200",
              transparent
                ? "border-white/70 bg-black/20 text-white backdrop-blur-sm group-hover:bg-[#e84d7a]"
                : "border-ink bg-ink text-paper group-hover:border-sakura-deep group-hover:bg-sakura-deep",
            )}
          >
            <Code2 className="size-4" />
          </span>
          <span
            className={cn(
              "font-playful text-xl font-bold tracking-[-0.02em] transition-colors sm:text-2xl",
              transparent ? "text-white [text-shadow:0_2px_9px_rgba(0,0,0,0.4)]" : "text-ink",
            )}
          >
            Link start！
          </span>
        </Link>

        <nav className="ml-2 hidden items-center gap-1 md:flex" aria-label="主导航">
          {[
            { href: "/", label: "首页" },
            { href: "/#latest", label: "文章" },
            { href: "/about", label: "关于" },
          ].map((item) => (
            <Link
              key={item.href}
              href={item.href}
              className={cn(
                "group px-3 py-2 font-playful text-base font-bold tracking-wide transition-colors",
                navTextClass,
              )}
            >
              <span className="underline-sweep">{item.label}</span>
            </Link>
          ))}
        </nav>

        <div className="ml-auto flex items-center gap-1.5 sm:gap-2">
          {transparent ? (
            <Link
              href="/search"
              aria-label="搜索文章"
              className="grid size-9 place-items-center rounded-md text-white transition-colors hover:bg-white/15"
            >
              <Search className="size-5" />
            </Link>
          ) : (
            <div className="hidden w-[min(30vw,320px)] sm:block">
              <SearchBox />
            </div>
          )}

          <ThemeToggle inverted={transparent} />

          {isLoggedIn ? (
            <>
              {isAdmin ? (
                <>
                  <Link
                    href="/admin"
                    className={cn(
                      buttonVariants({ variant: "ghost", size: "sm" }),
                      "font-playful hidden font-bold sm:inline-flex",
                      transparent
                        ? "text-white hover:bg-white/15 hover:text-white"
                        : "text-ink hover:bg-sakura-wash hover:text-sakura-deep",
                    )}
                  >
                    管理后台
                  </Link>
                  <Link
                    href="/editor"
                    className={cn(
                      buttonVariants({ size: "sm" }),
                      "font-playful hidden rounded-[4px] font-bold sm:inline-flex",
                      transparent
                        ? "bg-[#17171d]/90 text-white hover:bg-sakura-deep"
                        : "bg-ink text-paper hover:bg-sakura-deep",
                    )}
                  >
                    <PenSquare className="size-4" />
                    写文章
                  </Link>
                </>
              ) : null}
              <div className={transparent ? "[&_a]:text-white [&_a:hover]:bg-white/15" : undefined}>
                <NotificationBell />
              </div>
              <UserMenu />
            </>
          ) : (
            <>
              <Link
                href="/login"
                className={cn(
                  buttonVariants({ variant: "ghost", size: "sm" }),
                  "font-playful font-bold",
                  transparent
                    ? "text-white hover:bg-white/15 hover:text-white"
                    : "text-ink hover:bg-sakura-wash hover:text-sakura-deep",
                )}
              >
                登录
              </Link>
              <Link
                href="/register"
                className={cn(
                  buttonVariants({ size: "sm" }),
                  "font-playful hidden rounded-[4px] font-bold sm:inline-flex",
                  transparent
                    ? "bg-[#17171d]/90 text-white hover:bg-sakura-deep"
                    : "bg-ink text-paper hover:bg-sakura-deep",
                )}
              >
                注册
              </Link>
            </>
          )}
        </div>
      </Container>
    </header>
  )
}
