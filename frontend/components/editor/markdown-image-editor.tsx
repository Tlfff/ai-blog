"use client"

import type { ClipboardEvent, ChangeEvent } from "react"
import { ImageIcon, Loader2, RotateCcw, Trash2 } from "lucide-react"
import { cn } from "@/lib/utils"

export interface ImagePreview {
  previewUrl: string
  fileName: string
  status: "uploading" | "success" | "failed" | "saved"
  error?: string
}

interface MarkdownImageEditorProps {
  value: string
  disabled?: boolean
  imagePreviews: Map<string, ImagePreview>
  onChange: (value: string) => void
  onPasteImage: (file: File, start: number, end: number) => void
  onRemoveImage: (source: string) => void
  onRetryImage: (source: string) => void
}

interface TextBlock {
  type: "text"
  value: string
  start: number
  end: number
}

interface ImageBlock {
  type: "image"
  markdown: string
  alt: string
  src: string
  start: number
  end: number
}

type EditorBlock = TextBlock | ImageBlock

const MARKDOWN_IMAGE_PATTERN = /!\[([^\]]*)\]\(([^)\s]+)(?:\s+["'][^"']*["'])?\)/g

function parseEditorBlocks(content: string): EditorBlock[] {
  const blocks: EditorBlock[] = []
  let textStart = 0

  for (const match of content.matchAll(MARKDOWN_IMAGE_PATTERN)) {
    const start = match.index ?? 0
    if (start > textStart) {
      blocks.push({
        type: "text",
        value: content.slice(textStart, start),
        start: textStart,
        end: start,
      })
    }

    blocks.push({
      type: "image",
      markdown: match[0],
      alt: match[1] || "文章图片",
      src: match[2],
      start,
      end: start + match[0].length,
    })
    textStart = start + match[0].length
  }

  blocks.push({
    type: "text",
    value: content.slice(textStart),
    start: textStart,
    end: content.length,
  })

  return blocks
}

export function MarkdownImageEditor({
  value,
  disabled = false,
  imagePreviews,
  onChange,
  onPasteImage,
  onRemoveImage,
  onRetryImage,
}: MarkdownImageEditorProps) {
  const blocks = parseEditorBlocks(value)

  function updateTextBlock(block: TextBlock, event: ChangeEvent<HTMLTextAreaElement>) {
    onChange(`${value.slice(0, block.start)}${event.target.value}${value.slice(block.end)}`)
  }

  function handlePaste(block: TextBlock, event: ClipboardEvent<HTMLTextAreaElement>) {
    const imageItem = Array.from(event.clipboardData.items).find((item) => item.type.startsWith("image/"))
    if (!imageItem) return

    const file = imageItem.getAsFile()
    if (!file) return

    event.preventDefault()
    const selectionStart = event.currentTarget.selectionStart ?? block.value.length
    const selectionEnd = event.currentTarget.selectionEnd ?? block.value.length
    onPasteImage(file, block.start + selectionStart, block.start + selectionEnd)
  }

  function removeImage(block: ImageBlock) {
    onChange(`${value.slice(0, block.start)}${value.slice(block.end)}`)
    onRemoveImage(block.src)
  }

  function getTextAreaRows(block: TextBlock) {
    const visibleContent = block.value.replace(/^\n+|\n+$/g, "")
    const visibleLineCount = visibleContent ? visibleContent.split("\n").length : 1
    return blocks.length === 1 ? Math.max(6, visibleLineCount) : Math.max(1, visibleLineCount)
  }

  return (
    <div className="overflow-hidden rounded-sm border border-border bg-paper-deep focus-within:border-sakura-deep focus-within:ring-2 focus-within:ring-sakura/30">
      {blocks.map((block, index) => {
        if (block.type === "text") {
          return (
            <textarea
              key={`text-${block.start}-${index}`}
              value={block.value}
              onChange={(event) => updateTextBlock(block, event)}
              onPaste={(event) => handlePaste(block, event)}
              disabled={disabled}
              placeholder={blocks.length === 1 ? "在这里输入文章内容，支持 Markdown 格式..." : "继续写作..."}
              rows={getTextAreaRows(block)}
              className={cn(
                "block w-full border-0 bg-transparent px-4 font-mono text-sm leading-7 text-ink outline-none placeholder:text-ink-soft/60 disabled:cursor-not-allowed disabled:opacity-60",
                blocks.length === 1 ? "min-h-44 resize-y py-4" : "min-h-0 resize-none py-2.5",
              )}
            />
          )
        }

        const imagePreview = imagePreviews.get(block.src)
        const displaySource = imagePreview?.previewUrl ?? block.src
        const missingManagedImage = block.src.startsWith("image://") && !imagePreview
        const statusLabel = imagePreview
          ? {
              uploading: "上传中",
              success: "已上传，等待保存",
              failed: "上传失败",
              saved: "已绑定",
            }[imagePreview.status]
          : null

        return (
          <figure
            key={`image-${block.start}-${block.src}`}
            className="group relative mx-3 my-1.5 overflow-hidden rounded-sm bg-paper"
          >
            <div className="flex items-center justify-center bg-paper-deep/45 p-2">
              {missingManagedImage ? (
                <div className="px-5 py-10 text-center text-sm text-ink-soft">
                  图片映射不可用：{block.src}
                </div>
              ) : (
                // eslint-disable-next-line @next/next/no-img-element
                <img
                  src={displaySource}
                  alt={block.alt}
                  className="max-h-[34rem] max-w-full rounded-[2px] object-contain"
                />
              )}
            </div>
            <figcaption className="flex items-center justify-between gap-3 border-t border-border/60 px-2.5 py-1.5 text-xs text-ink-soft">
              <span className="flex min-w-0 items-center gap-2">
                <ImageIcon className="size-3.5 shrink-0 text-sakura-deep" />
                <span className="truncate">{imagePreview?.fileName || block.alt}</span>
                {statusLabel && (
                  <span className="inline-flex shrink-0 items-center gap-1 rounded-full bg-sakura-wash px-2 py-0.5 text-sakura-deep">
                    {imagePreview?.status === "uploading" && <Loader2 className="size-3 animate-spin" />}
                    {statusLabel}
                  </span>
                )}
                {imagePreview?.error && (
                  <span className="truncate text-destructive" title={imagePreview.error}>
                    {imagePreview.error}
                  </span>
                )}
              </span>
              <span className="flex shrink-0 items-center gap-3">
                {imagePreview?.status === "failed" && (
                  <button
                    type="button"
                    onClick={() => onRetryImage(block.src)}
                    disabled={disabled}
                    className="inline-flex items-center gap-1 text-sakura-deep transition-colors hover:text-ink disabled:cursor-not-allowed disabled:opacity-50"
                  >
                    <RotateCcw className="size-3.5" />
                    重试
                  </button>
                )}
                <button
                  type="button"
                  onClick={() => removeImage(block)}
                  disabled={disabled}
                  className="inline-flex items-center gap-1 text-ink-soft transition-colors hover:text-destructive disabled:cursor-not-allowed disabled:opacity-50"
                >
                  <Trash2 className="size-3.5" />
                  删除
                </button>
              </span>
            </figcaption>
          </figure>
        )
      })}
    </div>
  )
}
