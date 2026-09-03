"use client"

import { useEffect, useRef, useState } from "react"
import { useRouter, useSearchParams } from "next/navigation"
import { ArrowLeft, ImagePlus, Save, Send, Sparkles } from "lucide-react"
import useSWR from "swr"
import { Container } from "@/components/layout/container"
import { SiteShell } from "@/components/layout/site-shell"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import {
  MarkdownImageEditor,
  type ImagePreview,
} from "@/components/editor/markdown-image-editor"
import { useAuth } from "@/hooks/use-auth"
import {
  createArticle,
  getArticleById,
  getArticleImageUploadURL,
  getTags,
  updateArticle,
  uploadArticleImage,
} from "@/api/articles"
import { cn } from "@/lib/utils"

const ALLOWED_IMAGE_EXTENSIONS = new Set(["jpg", "jpeg", "png", "webp"])
const IMAGE_MIME_EXTENSIONS: Record<string, string> = {
  "image/jpeg": "jpg",
  "image/png": "png",
  "image/webp": "webp",
}

type PendingImageStatus = "uploading" | "success" | "failed" | "saved"

interface PendingImage {
  clientId: string
  file: File
  fileExt: string
  temporarySource: string
  previewUrl: string
  status: PendingImageStatus
  systemSource?: string
  publicUrl?: string
  error?: string
}

interface SubmissionProgress {
  value: number
  label: string
}

function getImageFileExtension(file: File): string | null {
  const filenameExtension = file.name.includes(".") ? file.name.split(".").pop()?.toLowerCase() : undefined
  if (filenameExtension) {
    return ALLOWED_IMAGE_EXTENSIONS.has(filenameExtension) ? filenameExtension : null
  }
  return IMAGE_MIME_EXTENSIONS[file.type.toLowerCase()] ?? null
}

export default function EditorPage() {
  const { isLoggedIn } = useAuth()
  const router = useRouter()
  const searchParams = useSearchParams()
  const articleId = searchParams.get("id")

  const [title, setTitle] = useState("")
  const [content, setContent] = useState("")
  const [selectedTags, setSelectedTags] = useState<string[]>([])
  const [saving, setSaving] = useState(false)
  const [publishing, setPublishing] = useState(false)
  const [pendingImages, setPendingImages] = useState<PendingImage[]>([])
  const [submissionProgress, setSubmissionProgress] = useState<SubmissionProgress | null>(null)
  const [uploadMessage, setUploadMessage] = useState("")
  const fileInputRef = useRef<HTMLInputElement>(null)
  const previewUrlsRef = useRef(new Set<string>())
  const isSubmitting = saving || publishing
  const uploadingImageCount = pendingImages.filter(
    (image) => image.status === "uploading" && content.includes(image.temporarySource),
  ).length
  const failedImageCount = pendingImages.filter(
    (image) => image.status === "failed" && content.includes(image.temporarySource),
  ).length
  const { data: article, isLoading } = useSWR(
    articleId ? ["article", articleId] : null,
    () => getArticleById(articleId!),
  )

  const { data: availableTags = [] } = useSWR("editor-tags", () => getTags())
  const imagePreviews = new Map<string, ImagePreview>(
    (article?.images ?? []).map((image) => [
      `image://${image.id}`,
      { previewUrl: image.url, fileName: `文章图片 #${image.id}`, status: "saved" as const },
    ]),
  )
  pendingImages.forEach((image) => {
    imagePreviews.set(image.systemSource ?? image.temporarySource, {
      previewUrl: image.publicUrl ?? image.previewUrl,
      fileName: image.file.name || "文章图片",
      status: image.status,
      error: image.error,
    })
  })

  useEffect(() => {
    if (!isLoggedIn) {
      router.push("/")
    }
  }, [isLoggedIn, router])

  useEffect(() => {
    if (article) {
      setTitle(article.title)
      setContent(article.content)
      setSelectedTags(article.tags.map((t) => t.name))
    }
  }, [article])

  useEffect(() => {
    if (!isSubmitting && uploadingImageCount === 0) return

    const handleBeforeUnload = (event: BeforeUnloadEvent) => {
      event.preventDefault()
      event.returnValue = ""
    }

    window.addEventListener("beforeunload", handleBeforeUnload)
    return () => window.removeEventListener("beforeunload", handleBeforeUnload)
  }, [isSubmitting, uploadingImageCount])

  useEffect(() => {
    const previewUrls = previewUrlsRef.current
    return () => {
      previewUrls.forEach((url) => URL.revokeObjectURL(url))
      previewUrls.clear()
    }
  }, [])

  if (!isLoggedIn) {
    return null
  }

  if (articleId && isLoading) {
    return (
      <SiteShell>
        <Container className="py-6">
          <div className="text-center py-12">加载中...</div>
        </Container>
      </SiteShell>
    )
  }

  function toggleTag(tagName: string) {
    setSelectedTags((prev) =>
      prev.includes(tagName) ? prev.filter((t) => t !== tagName) : [...prev, tagName],
    )
  }

  function releaseImagePreview(previewUrl: string) {
    URL.revokeObjectURL(previewUrl)
    previewUrlsRef.current.delete(previewUrl)
  }

  async function uploadPendingImage(image: PendingImage) {
    setPendingImages((current) =>
      current.map((item) =>
        item.clientId === image.clientId
          ? { ...item, status: "uploading", error: undefined }
          : item,
      ),
    )
    setUploadMessage(`正在上传 ${image.file.name || "文章图片"}...`)

    try {
      const credential = await getArticleImageUploadURL(image.fileExt)
      await uploadArticleImage(image.file, credential.upload_url)

      const systemSource = `image://${credential.image_id}`
      setContent((current) => current.split(image.temporarySource).join(systemSource))
      setPendingImages((current) =>
        current.map((item) =>
          item.clientId === image.clientId
            ? {
                ...item,
                status: "success",
                systemSource,
                publicUrl: credential.url,
                error: undefined,
              }
            : item,
        ),
      )
      releaseImagePreview(image.previewUrl)
      setUploadMessage("图片上传成功，保存文章时会自动绑定")
    } catch (error) {
      const message = error instanceof Error ? error.message : "图片上传失败"
      setPendingImages((current) =>
        current.map((item) =>
          item.clientId === image.clientId
            ? { ...item, status: "failed", error: message }
            : item,
        ),
      )
      setUploadMessage(`图片上传失败：${message}`)
    }
  }

  function insertImage(file: File, start?: number, end?: number) {
    const fileExt = getImageFileExtension(file)
    if (!fileExt) {
      setUploadMessage("仅支持 JPG、JPEG、PNG 和 WebP 图片")
      return
    }

    const clientId = crypto.randomUUID()
    const temporarySource = `blog-local-image://${clientId}`
    const previewUrl = URL.createObjectURL(file)
    previewUrlsRef.current.add(previewUrl)
    const imageName = (file.name || "文章图片").replace(/[\[\]]/g, "")
    const markdown = `\n\n![${imageName}](${temporarySource})\n\n`
    const pendingImage: PendingImage = {
      clientId,
      file,
      fileExt,
      temporarySource,
      previewUrl,
      status: "uploading",
    }

    setPendingImages((current) => [...current, pendingImage])
    setContent((current) => {
      const insertionStart = start ?? current.length
      const insertionEnd = end ?? insertionStart
      return `${current.slice(0, insertionStart)}${markdown}${current.slice(insertionEnd)}`
    })
    void uploadPendingImage(pendingImage)
  }

  function removePendingImage(source: string) {
    const removedImage = pendingImages.find(
      (image) => image.temporarySource === source || image.systemSource === source,
    )
    if (removedImage) {
      releaseImagePreview(removedImage.previewUrl)
    }
    setPendingImages((current) => current.filter((image) => image.clientId !== removedImage?.clientId))
  }

  function retryPendingImage(source: string) {
    const image = pendingImages.find(
      (item) => item.temporarySource === source || item.systemSource === source,
    )
    if (image) void uploadPendingImage(image)
  }

  async function submitArticle(status: 2 | 3): Promise<string> {
    const contentSnapshot = content.trim()
    if (uploadingImageCount > 0) {
      throw new Error("仍有图片正在上传，请稍候")
    }
    if (failedImageCount > 0 || contentSnapshot.includes("blog-local-image://")) {
      throw new Error("正文中存在上传失败的图片，请重试或删除后再保存")
    }

    setUploadMessage(status === 2 ? "正在保存草稿..." : "正在发布文章...")
    setSubmissionProgress({ value: 70, label: status === 2 ? "正在保存草稿" : "正在发布文章" })
    const articleInput = {
      title: title.trim(),
      content: contentSnapshot,
      tags: selectedTags,
      status,
    }
    let targetArticleId = articleId
    if (targetArticleId) {
      await updateArticle(targetArticleId, articleInput)
    } else {
      targetArticleId = await createArticle(articleInput)
    }
    setPendingImages((current) =>
      current.map((image) =>
        image.systemSource && contentSnapshot.includes(image.systemSource)
          ? { ...image, status: "saved" }
          : image,
      ),
    )

    setUploadMessage(status === 2 ? "草稿已保存" : "文章已发布")
    setSubmissionProgress({ value: 100, label: status === 2 ? "草稿保存完成" : "文章发布完成" })
    return targetArticleId
  }

  async function handleSaveDraft() {
    if (!title.trim()) {
      alert("请输入文章标题")
      return
    }
    if (!content.trim()) {
      alert("请输入文章内容")
      return
    }
    setSaving(true)
    setSubmissionProgress({ value: 5, label: "正在准备保存草稿" })
    try {
      const savedArticleId = await submitArticle(2)
      if (!articleId) {
        router.replace(`/editor?id=${savedArticleId}`)
      }
      alert("草稿保存成功")
    } catch (error) {
      const message = error instanceof Error ? error.message : "草稿保存失败"
      setUploadMessage(message)
      alert(`草稿保存失败：${message}`)
    } finally {
      setSubmissionProgress(null)
      setSaving(false)
    }
  }

  async function handlePublish() {
    if (!title.trim()) {
      alert("请输入文章标题")
      return
    }
    if (!content.trim()) {
      alert("请输入文章内容")
      return
    }
    setPublishing(true)
    setSubmissionProgress({ value: 5, label: "正在准备发布文章" })
    try {
      const publishedArticleId = await submitArticle(3)
      router.push(`/articles/${publishedArticleId}`)
    } catch (error) {
      const message = error instanceof Error ? error.message : "文章发布失败"
      setUploadMessage(message)
      alert(`文章发布失败：${message}`)
    } finally {
      setSubmissionProgress(null)
      setPublishing(false)
    }
  }

  function handleBack() {
    router.back()
  }

  return (
    <SiteShell>
      <Container className="py-8 md:py-12">
        <div className="mx-auto max-w-6xl">
          <div className="mb-8 flex flex-col gap-5 border-b border-border pb-6 md:flex-row md:items-end md:justify-between">
            <div className="flex items-center gap-3">
              <button
                onClick={handleBack}
                disabled={isSubmitting}
                className="label-meta inline-flex items-center gap-2 text-ink transition-colors hover:text-sakura-deep disabled:cursor-not-allowed disabled:opacity-50"
              >
                <ArrowLeft className="size-4" />
                back
              </button>
              <div>
                <p className="label-meta text-sakura-deep">studio / writing desk</p>
                <h1 className="title-display mt-2 text-3xl text-ink">{articleId ? "编辑文章" : "写文章"}</h1>
              </div>
            </div>

            <div className="flex items-center gap-2">
              <Button
                variant="outline"
                size="sm"
                onClick={handleSaveDraft}
                disabled={isSubmitting || uploadingImageCount > 0}
              >
                <Save className="size-4" />
                {saving ? "保存中..." : "保存草稿"}
              </Button>
              <Button
                size="sm"
                onClick={handlePublish}
                disabled={isSubmitting || uploadingImageCount > 0}
              >
                <Send className="size-4" />
                {publishing ? "发布中..." : articleId ? "更新" : "发布"}
              </Button>
            </div>
          </div>

          {isSubmitting && submissionProgress && (
            <div className="mb-8 border border-sakura/60 bg-sakura-wash px-4 py-4 sm:px-5" aria-live="polite">
              <div className="mb-2 flex items-center justify-between gap-4 text-sm">
                <span className="font-medium text-ink">{submissionProgress.label}</span>
                <span className="label-meta text-sakura-deep">{submissionProgress.value}%</span>
              </div>
              <div
                role="progressbar"
                aria-label={submissionProgress.label}
                aria-valuemin={0}
                aria-valuemax={100}
                aria-valuenow={submissionProgress.value}
                className="h-2 overflow-hidden rounded-full bg-paper-deep"
              >
                <div
                  className="h-full rounded-full bg-sakura-deep transition-[width] duration-300 ease-out"
                  style={{ width: `${submissionProgress.value}%` }}
                />
              </div>
              <p className="mt-2 text-xs text-ink-soft">上传和保存完成前请不要刷新或关闭页面。</p>
            </div>
          )}

          <div className="grid gap-8 lg:grid-cols-[minmax(0,1fr)_260px]">
            <div className="card-paper overflow-hidden rounded-sm">
              <div className="border-b border-border bg-paper-deep px-5 py-4 sm:px-7">
                <p className="label-meta text-sakura-deep">draft / untitled note</p>
                <input
                  type="text"
                  value={title}
                  onChange={(e) => setTitle(e.target.value)}
                  disabled={isSubmitting}
                  placeholder="文章标题"
                  className="title-display mt-3 w-full bg-transparent text-3xl text-ink outline-none placeholder:text-ink-soft/50 disabled:cursor-not-allowed disabled:opacity-60 md:text-4xl"
                />
              </div>

              <div className="flex flex-col gap-7 p-5 sm:p-7">
                <div>
                  <label className="label-meta mb-3 block text-sakura-deep">tags / 标签</label>
                  <div className="flex flex-wrap gap-2">
                    {availableTags.map((tag) => (
                      <button
                        key={tag.id}
                        type="button"
                        onClick={() => toggleTag(tag.name)}
                        disabled={isSubmitting}
                        className={cn(
                          "rounded-sm border px-3 py-1.5 text-sm transition-colors disabled:cursor-not-allowed disabled:opacity-50",
                          selectedTags.includes(tag.name)
                            ? "border-sakura-deep bg-sakura-deep text-primary-foreground"
                            : "border-border bg-transparent text-ink-soft hover:border-sakura-deep hover:text-sakura-deep",
                        )}
                      >
                        {tag.name}
                      </button>
                    ))}
                  </div>
                </div>

                <div>
                  <div className="mb-3 flex items-center justify-between">
                    <label className="label-meta whitespace-nowrap text-sakura-deep">body / markdown</label>
                    <div className="flex items-center gap-3">
                      <span className="label-meta hidden text-ink-soft sm:inline">paste or select image</span>
                      <input
                        ref={fileInputRef}
                        type="file"
                        accept="image/jpeg,image/png,image/webp"
                        multiple
                        className="hidden"
                        onChange={(event) => {
                          Array.from(event.target.files ?? []).forEach((file) => insertImage(file))
                          event.target.value = ""
                        }}
                      />
                      <button
                        type="button"
                        onClick={() => fileInputRef.current?.click()}
                        disabled={isSubmitting}
                        className="inline-flex shrink-0 items-center gap-1 text-xs text-sakura-deep transition-colors hover:text-ink disabled:cursor-not-allowed disabled:opacity-50"
                      >
                        <ImagePlus className="size-3.5 sm:hidden" />
                        选择图片
                      </button>
                    </div>
                  </div>
                  <MarkdownImageEditor
                    value={content}
                    onChange={setContent}
                    disabled={isSubmitting}
                    imagePreviews={imagePreviews}
                    onPasteImage={insertImage}
                    onRemoveImage={removePendingImage}
                    onRetryImage={retryPendingImage}
                  />
                  <div className="mt-3 flex items-center gap-2 text-xs text-ink-soft">
                    <ImagePlus className="size-3.5 text-sakura-deep" />
                    <span>图片会立即上传，正文保存稳定的 image:// 图片引用</span>
                    {uploadingImageCount > 0 && !isSubmitting && (
                      <span className="text-sakura-deep">上传中 {uploadingImageCount} 张</span>
                    )}
                    {failedImageCount > 0 && !isSubmitting && (
                      <span className="text-destructive">失败 {failedImageCount} 张</span>
                    )}
                    {uploadMessage && <span>{uploadMessage}</span>}
                  </div>
                </div>

                <div className="border-l-2 border-sakura bg-sakura-wash px-4 py-3">
                  <p className="text-xs leading-6 text-ink-soft">
                    支持 Markdown 语法：标题、列表、代码块、引用等。使用 # 表示标题，**粗体**，*斜体*。
                  </p>
                </div>
              </div>
            </div>

            <aside className="hidden lg:block">
              <div className="sticky top-24 space-y-5">
                <div className="border-y border-border py-5">
                  <p className="label-meta text-sakura-deep">writing notes</p>
                  <div className="mt-4 space-y-3 text-sm leading-7 text-ink-soft">
                    <p>标题先说清楚，再让正文慢慢展开。</p>
                    <p>摘要会根据正文自动生成，无需单独填写。</p>
                    <p>图片粘贴或选择后立即上传，文章正文只保存稳定图片引用。</p>
                  </div>
                </div>
                <div className="bg-sakura-wash p-5">
                  <Sparkles className="size-5 text-sakura-deep" />
                  <p className="mt-3 text-sm leading-7 text-ink-soft">
                    一篇好文章不必一次完成，先保存下来，之后再回来继续连接。
                  </p>
                </div>
              </div>
            </aside>
          </div>
        </div>
      </Container>
    </SiteShell>
  )
}
