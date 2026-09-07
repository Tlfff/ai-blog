import { ArticleList } from "@/components/article/article-list"
import { HomeSidebar } from "@/components/article/home-sidebar"
import { HomeHero } from "@/components/home/home-hero"
import { Container } from "@/components/layout/container"
import { SiteShell } from "@/components/layout/site-shell"
import { getHomeImageCatalog } from "@/lib/server/image-catalog"

function SectionHeading({ children }: { children: React.ReactNode }) {
  return (
    <div className="mb-8 flex items-center gap-4 sm:mb-10 sm:gap-6">
      <span className="h-px flex-1 bg-[var(--home-divider)]" aria-hidden />
      <h2 className="font-playful shrink-0 text-2xl font-bold tracking-wide text-[var(--home-text)] sm:text-3xl">
        {children}
      </h2>
      <span className="h-px flex-1 bg-[var(--home-divider)]" aria-hidden />
    </div>
  )
}

export default async function HomePage({
  searchParams,
}: {
  searchParams: Promise<{ tag?: string; tab?: string }>
}) {
  const [{ tag }, { carouselSlides, articleFallbackCovers }] = await Promise.all([
    searchParams,
    getHomeImageCatalog(),
  ])

  return (
    <SiteShell immersiveHeader>
      <HomeHero slides={carouselSlides} />
      <section className="relative bg-[var(--home-surface)] text-[var(--home-text)] transition-colors duration-300">
        <Container className="max-w-[1240px] pb-16 pt-7 sm:pb-20 sm:pt-10 lg:pb-24">
          <div className="grid gap-12 lg:grid-cols-[240px_minmax(0,1fr)] lg:gap-14 xl:gap-20">
            <aside id="about" className="scroll-mt-24 lg:sticky lg:top-20 lg:max-h-[calc(100dvh-6rem)] lg:self-start lg:overflow-y-auto lg:overscroll-contain lg:pr-2">
              <HomeSidebar />
            </aside>

            <main id="latest" className="min-w-0 scroll-mt-24">
              <SectionHeading>{tag ? `# ${tag}` : "最新文章"}</SectionHeading>
              <ArticleList tag={tag} fallbackCovers={articleFallbackCovers} />
            </main>
          </div>

          <footer className="mt-16 flex items-center gap-5 border-t border-[var(--home-divider)] pt-6 text-xs text-[var(--home-faint)]">
            <span>© 2026 睦子米</span>
            <span className="h-px flex-1 bg-[var(--home-divider)]" aria-hidden />
            <span className="font-playful text-sm text-[var(--home-muted)]">Link start！</span>
          </footer>
        </Container>
      </section>
    </SiteShell>
  )
}
