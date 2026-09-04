"use client"

import ReactMarkdown from "react-markdown"
import remarkGfm from "remark-gfm"
import type { ArticleImage } from "@/types"
import { slugifyHeading } from "./article-toc"

interface ArticleContentProps {
  content: string
  images?: ArticleImage[]
}

function resolveArticleImageURLs(content: string, images: ArticleImage[]) {
  const imageURLs = new Map(images.map((image) => [image.id, image.url]))
  return content.replace(
    /(!\[[^\]]*\]\()image:\/\/(\d+)((?:\s+["'][^"']*["'])?\))/g,
    (markdown, prefix: string, imageID: string, suffix: string) => {
      const url = imageURLs.get(imageID)
      return url ? `${prefix}${url}${suffix}` : markdown
    },
  )
}

export function ArticleContent({ content, images = [] }: ArticleContentProps) {
  const renderedContent = resolveArticleImageURLs(content, images)

  return (
    <div className="prose-container">
      <ReactMarkdown
        remarkPlugins={[remarkGfm]}
        components={{
          h1: ({ children }) => (
            <h1 id={slugifyHeading(String(children))} className="scroll-mt-24 font-playful mt-10 mb-6 text-3xl font-bold text-foreground pb-3 border-b border-primary/40">
              {children}
            </h1>
          ),
          h2: ({ children }) => (
            <h2 id={slugifyHeading(String(children))} className="scroll-mt-24 font-playful mt-8 mb-4 text-2xl font-bold text-foreground pb-2 border-b border-primary/30">
              {children}
            </h2>
          ),
          h3: ({ children }) => (
            <h3 id={slugifyHeading(String(children))} className="scroll-mt-24 font-playful mt-6 mb-3 text-xl font-bold text-foreground">
              {children}
            </h3>
          ),
          h4: ({ children }) => (
            <h4 className="font-playful mt-5 mb-2 text-lg font-bold text-foreground">
              {children}
            </h4>
          ),
          h5: ({ children }) => (
            <h5 className="font-playful mt-4 mb-2 text-base font-bold text-foreground">
              {children}
            </h5>
          ),
          h6: ({ children }) => (
            <h6 className="mt-4 mb-2 text-sm font-semibold text-muted-foreground">
              {children}
            </h6>
          ),
          p: ({ children }) => (
            <p className="my-5 text-base leading-8 text-foreground">
              {children}
            </p>
          ),
          strong: ({ children }) => (
            <strong className="font-semibold text-accent">{children}</strong>
          ),
          em: ({ children }) => (
            <em className="italic text-foreground">{children}</em>
          ),
          code: ({ className, children }) => {
            const isInline = !className
            if (isInline) {
              return (
                <code className="rounded-md border border-[var(--article-code-border)] bg-[var(--article-code-surface)] px-2 py-0.5 font-mono text-sm text-[var(--article-accent)]">
                  {children}
                </code>
              )
            }
            return (
              <code className={`${className} block max-w-full language-${className?.replace('language-', '') || 'plaintext'}`}>
                {children}
              </code>
            )
          },
          pre: ({ children }) => (
            <pre className="my-5 overflow-x-auto rounded-lg border border-[var(--article-code-border)] bg-[var(--article-code-surface)] p-5 font-mono text-sm text-[var(--article-code-text)] shadow-none">
              {children}
            </pre>
          ),
          ul: ({ children }) => (
            <ul className="my-5 list-disc list-outside pl-7 text-foreground">
              {children}
            </ul>
          ),
          ol: ({ children }) => (
            <ol className="my-5 list-decimal list-outside pl-7 text-foreground">
              {children}
            </ol>
          ),
          li: ({ children }) => (
            <li className="my-3 text-foreground">{children}</li>
          ),
          blockquote: ({ children }) => (
            <blockquote className="my-5 rounded-r-lg border-l-4 border-[var(--article-accent)] bg-[var(--article-summary)] py-3 pl-5 italic text-muted-foreground">
              {children}
            </blockquote>
          ),
          a: ({ href, children }) => (
            <a
              href={href}
              className="text-accent border-b border-accent/30 hover:text-accent/80 hover:border-accent transition-all duration-200"
              target="_blank"
              rel="noopener noreferrer"
            >
              {children}
            </a>
          ),
          img: ({ src, alt }) => (
            <img
              src={src}
              alt={alt}
              className="my-6 max-w-full rounded-lg border border-[var(--article-divider)] shadow-none"
            />
          ),
          hr: () => (
            <hr className="my-10 h-px border-none bg-[var(--article-divider)]" />
          ),
          table: ({ children }) => (
            <div className="my-5 overflow-x-auto rounded-xl border border-border overflow-hidden">
                <table className="w-full border-collapse bg-[var(--article-surface)] text-sm">
                {children}
              </table>
            </div>
          ),
          th: ({ children }) => (
            <th className="border border-border bg-secondary px-4 py-3 text-left font-semibold text-foreground">
              {children}
            </th>
          ),
          td: ({ children }) => (
            <td className="px-4 py-3 border border-border text-foreground">
              {children}
            </td>
          ),
        }}
      >
        {renderedContent}
      </ReactMarkdown>
    </div>
  )
}
