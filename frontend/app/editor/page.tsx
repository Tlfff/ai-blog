"use client"

import { useEffect, useRef, useState } from "react"
import Image from "next/image"
import { useRouter, useSearchParams } from "next/navigation"
import { ArrowLeft, CheckCircle2, FileText, ImagePlus, Save, Send, Tag } from "lucide-react"
import useSWR from "swr"
import { Container } from "@/components/layout/container"
import { SiteShell } from "@/components/layout/site-shell"
import { Button } from "@/components/ui/button"
import {
  WysiwygMarkdownEditor,
  type PreparedEditorImage,
  type WysiwygImagePreview,
  type WysiwygMarkdownEditorHandle,
} from "@/components/editor/wysiwyg-markdown-editor"
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

const DEFAULT_TAGS = ["Go", "后端", "前端", "数据库", "AI", "工程实践", "生活随笔"]

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
  const editorRef = useRef<WysiwygMarkdownEditorHandle>(null)
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
  const imagePreviews = new Map<string, WysiwygImagePreview>(
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
  const displayedTags = Array.from(
    new Set([...DEFAULT_TAGS, ...availableTags.map((tag) => tag.name), ...selectedTags]),
  )
  const wordCount = content.replace(/!\[[^\]]*\]\([^)]*\)/g, "").replace(/\s/g, "").length
  const contentImageCount = (content.match(/!\[[^\]]*\]\([^)]*\)/g) ?? []).length

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

  useEffect(() => {
    const handleShortcut = (event: KeyboardEvent) => {
      if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === "s") {
        event.preventDefault()
        if (!isSubmitting && uploadingImageCount === 0) void handleSaveDraft()
      }
    }
    window.addEventListener("keydown", handleShortcut)
    return () => window.removeEventListener("keydown", handleShortcut)
  }, [content, isSubmitting, selectedTags, title, uploadingImageCount])

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

  function prepareImageUpload(file: File): PreparedEditorImage | null {
    const fileExt = getImageFileExtension(file)
    if (!fileExt) {
      setUploadMessage("仅支持 JPG、JPEG、PNG 和 WebP 图片")
      return null
    }

    const clientId = crypto.randomUUID()
    const temporarySource = `blog-local-image://${clientId}`
    const previewUrl = URL.createObjectURL(file)
    previewUrlsRef.current.add(previewUrl)
    const imageName = (file.name || "文章图片").replace(/[\[\]]/g, "")
    const pendingImage: PendingImage = {
      clientId,
      file,
      fileExt,
      temporarySource,
      previewUrl,
      status: "uploading",
    }

    setPendingImages((current) => [...current, pendingImage])
    void uploadPendingImage(pendingImage)
    return {
      source: temporarySource,
      previewUrl,
      fileName: file.name || "文章图片",
      alt: imageName,
    }
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
    <SiteShell immersiveHeader>
      <section className="editor-hero relative min-h-52 overflow-hidden">
        <Image
          src="/kv/bq-1.png"
          alt=""
          fill
          priority
          sizes="100vw"
          className="object-cover object-[center_48%]"
        />
        <div className="absolute inset-0 bg-[linear-gradient(180deg,rgba(20,55,92,0.42),rgba(25,73,111,0.3),rgba(238,247,251,0.94))] dark:bg-[linear-gradient(180deg,rgba(8,20,36,0.6),rgba(15,36,55,0.5),rgba(19,31,45,0.96))]" />
        <Container className="relative z-10 flex max-w-[1320px] flex-col gap-5 pb-10 pt-24 text-white sm:flex-row sm:items-end sm:justify-between">
          <div>
            <button
              type="button"
              onClick={handleBack}
              disabled={isSubmitting}
              className="inline-flex items-center gap-2 text-sm font-medium text-white/85 transition-colors hover:text-white disabled:opacity-50"
            >
              <ArrowLeft className="size-4" />
              返回
            </button>
            <div className="mt-3 flex flex-wrap items-center gap-3">
              <h1 className="font-playful text-4xl font-bold tracking-wide [text-shadow:0_2px_12px_rgba(0,0,0,0.28)]">
                {articleId ? "编辑文章" : "写文章"}
              </h1>
              <span className="inline-flex items-center gap-2 text-sm text-white/82">
                <span className="size-2 rounded-full bg-[#56c9be]" />
                所见即所得 · Markdown 保存
              </span>
            </div>
          </div>

          <div className="flex items-center gap-3">
            <Button
              variant="outline"
              onClick={handleSaveDraft}
              disabled={isSubmitting || uploadingImageCount > 0}
              className="rounded-full border-white/70 bg-white/12 text-white backdrop-blur-sm hover:bg-white hover:text-[#24405f]"
            >
              <Save className="size-4" />
              {saving ? "保存中..." : "保存草稿"}
            </Button>
            <Button
              onClick={handlePublish}
              disabled={isSubmitting || uploadingImageCount > 0}
              className="rounded-full bg-[#f47c78] px-6 text-white shadow-[0_8px_24px_rgba(244,124,120,0.28)] hover:bg-[#e86b68]"
            >
              <Send className="size-4" />
              {publishing ? "发布中..." : articleId ? "更新" : "发布"}
            </Button>
          </div>
        </Container>
      </section>

      <section className="editor-workspace min-h-[calc(100vh-13rem)] pb-16">
        <Container className="relative z-20 max-w-[1320px] -translate-y-5">
          {isSubmitting && submissionProgress ? (
            <div className="mb-5 rounded-xl border border-[var(--editor-border)] bg-[var(--editor-surface)] px-5 py-4 shadow-[0_14px_40px_var(--editor-shadow)]" aria-live="polite">
              <div className="mb-2 flex items-center justify-between text-sm">
                <span className="font-medium text-[var(--editor-ink)]">{submissionProgress.label}</span>
                <span className="text-[var(--editor-sky)]">{submissionProgress.value}%</span>
              </div>
              <div className="h-1.5 overflow-hidden rounded-full bg-[var(--editor-surface-muted)]">
                <div className="h-full rounded-full bg-[var(--editor-sky)] transition-[width] duration-300" style={{ width: `${submissionProgress.value}%` }} />
              </div>
            </div>
          ) : null}

          <div className="grid items-start gap-6 lg:grid-cols-[minmax(0,1fr)_286px]">
            <main className="editor-sheet min-w-0 overflow-hidden rounded-2xl border border-[var(--editor-border)] bg-[var(--editor-surface)] shadow-[0_18px_55px_var(--editor-shadow)]">
              <div className="px-5 pb-5 pt-7 sm:px-8 sm:pt-9">
                <p className="font-mono text-[11px] font-medium uppercase tracking-[0.2em] text-[var(--editor-muted)]">
                  draft / {articleId ? `article ${articleId}` : "untitled note"}
                </p>
                <input
                  type="text"
                  value={title}
                  onChange={(event) => setTitle(event.target.value)}
                  disabled={isSubmitting}
                  placeholder="文章标题"
                  className="font-playful mt-3 w-full border-0 bg-transparent text-3xl font-bold leading-tight text-[var(--editor-ink)] outline-none placeholder:text-[var(--editor-muted)]/55 disabled:opacity-60 sm:text-4xl"
                />
              </div>

              <div className="border-y border-[var(--editor-border)] px-5 py-5 sm:px-8">
                <div className="flex flex-wrap items-center justify-between gap-4">
                  <div>
                    <p className="mb-2 text-xs font-medium text-[var(--editor-muted)]">标签</p>
                    <div className="flex flex-wrap gap-2">
                      {displayedTags.map((tagName, index) => {
                        const selected = selectedTags.includes(tagName)
                        return (
                          <button
                            key={tagName}
                            type="button"
                            onClick={() => toggleTag(tagName)}
                            disabled={isSubmitting}
                            className={cn(
                              "rounded-full border px-3 py-1 text-sm transition-colors disabled:opacity-50",
                              selected
                                ? index % 2 === 0
                                  ? "border-[var(--editor-sky)] bg-[var(--editor-sky)] text-white"
                                  : "border-[var(--editor-teal)] bg-[var(--editor-teal)] text-white"
                                : "border-[var(--editor-border)] text-[var(--editor-muted)] hover:border-[var(--editor-sky)] hover:text-[var(--editor-sky)]",
                            )}
                          >
                            {tagName}
                            {selected ? " ×" : ""}
                          </button>
                        )
                      })}
                    </div>
                  </div>
                  <p className="text-xs text-[var(--editor-muted)]">摘要将在发布后根据正文自动生成</p>
                </div>
              </div>

              <div className="px-5 py-5 sm:px-8">
                <div className="mb-3 flex flex-wrap items-center justify-between gap-3">
                  <div>
                    <p className="font-mono text-xs font-medium uppercase tracking-[0.16em] text-[var(--editor-muted)]">正文 / wysiwyg</p>
                    <p className="mt-1 text-xs text-[var(--editor-muted)]">输入 ##、-、&gt; 或 ``` 后按空格即可转换，也可以输入 / 打开命令</p>
                  </div>
                  <div className="flex items-center gap-3">
                    <input
                      ref={fileInputRef}
                      type="file"
                      accept="image/jpeg,image/png,image/webp"
                      multiple
                      className="hidden"
                      onChange={(event) => {
                        Array.from(event.target.files ?? []).forEach((file) => editorRef.current?.insertImage(file))
                        event.target.value = ""
                      }}
                    />
                    <button
                      type="button"
                      onClick={() => fileInputRef.current?.click()}
                      disabled={isSubmitting}
                      className="inline-flex items-center gap-2 text-sm font-medium text-[var(--editor-sky)] transition-colors hover:text-[var(--editor-coral)] disabled:opacity-50"
                    >
                      <ImagePlus className="size-4" />
                      选择图片
                    </button>
                  </div>
                </div>

                <WysiwygMarkdownEditor
                  ref={editorRef}
                  value={content}
                  onChange={setContent}
                  disabled={isSubmitting}
                  imagePreviews={imagePreviews}
                  onPrepareImage={prepareImageUpload}
                  onRemoveImage={removePendingImage}
                  onRetryImage={retryPendingImage}
                />

                <div className="mt-3 flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-[var(--editor-muted)]">
                  <span className="inline-flex items-center gap-1.5"><ImagePlus className="size-3.5 text-[var(--editor-sky)]" />图片会立即上传，正文保存稳定的 image:// 图片引用</span>
                  {uploadingImageCount > 0 ? <span className="text-[var(--editor-sky)]">上传中 {uploadingImageCount} 张</span> : null}
                  {failedImageCount > 0 ? <span className="text-destructive">失败 {failedImageCount} 张</span> : null}
                  {uploadMessage ? <span>{uploadMessage}</span> : null}
                </div>
              </div>
            </main>

            <aside className="space-y-5 lg:sticky lg:top-20">
              <div className="overflow-hidden rounded-2xl border border-[var(--editor-border)] bg-[var(--editor-surface)] shadow-[0_14px_40px_var(--editor-shadow)]">
                <div className="relative aspect-[16/10] overflow-hidden">
                  <Image src="/kv/bq-1.png" alt="蓝天与云朵" fill sizes="286px" className="object-cover" />
                  <div className="absolute inset-0 bg-gradient-to-t from-[#173652]/75 via-transparent to-transparent" />
                  <span className="absolute bottom-3 left-4 font-mono text-xs font-semibold tracking-[0.14em] text-white">TODAY / SUMMER BLUE</span>
                </div>

                <div className="space-y-6 p-5">
                  <EditorRailSection title="写作状态">
                    <RailRow icon={<FileText className="size-4" />} color="sky" label="正文" value={`${wordCount.toLocaleString()} 字`} />
                    <RailRow icon={<ImagePlus className="size-4" />} color="teal" label="图片" value={`${contentImageCount} 张`} />
                    <RailRow icon={<Tag className="size-4" />} color="yellow" label="标签" value={`${selectedTags.length} 个`} />
                  </EditorRailSection>

                  <EditorRailSection title="发布检查">
                    <CheckRow done={Boolean(title.trim())} label="标题已填写" />
                    <CheckRow done={uploadingImageCount === 0 && failedImageCount === 0} label="图片上传完成" />
                    <CheckRow done={Boolean(content.trim())} label="正文内容完整" />
                    <CheckRow done label="摘要将自动生成" />
                  </EditorRailSection>

                  <EditorRailSection title="快捷键提示">
                    <p className="text-sm text-[var(--editor-muted)]"><kbd>⌘ S</kbd><span className="ml-3">保存草稿</span></p>
                    <p className="text-sm text-[var(--editor-muted)]"><kbd>/</kbd><span className="ml-3">打开内容命令</span></p>
                    <p className="text-sm text-[var(--editor-muted)]"><kbd>粘贴</kbd><span className="ml-3">图片即可上传</span></p>
                  </EditorRailSection>
                </div>
              </div>
            </aside>
          </div>
        </Container>
      </section>
    </SiteShell>
  )
}

function EditorRailSection({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <section>
      <div className="mb-3 flex items-center gap-3">
        <h2 className="font-playful shrink-0 text-lg font-bold text-[var(--editor-ink)]">{title}</h2>
        <span className="h-px flex-1 bg-[var(--editor-border)]" />
      </div>
      <div className="space-y-3">{children}</div>
    </section>
  )
}

function RailRow({ icon, color, label, value }: { icon: React.ReactNode; color: "sky" | "teal" | "yellow"; label: string; value: string }) {
  const colors = { sky: "text-[var(--editor-sky)]", teal: "text-[var(--editor-teal)]", yellow: "text-[var(--editor-yellow)]" }
  return (
    <div className="flex items-center gap-3 text-sm">
      <span className={colors[color]}>{icon}</span>
      <span className="text-[var(--editor-muted)]">{label}</span>
      <strong className="ml-auto text-[var(--editor-ink)]">{value}</strong>
    </div>
  )
}

function CheckRow({ done, label }: { done: boolean; label: string }) {
  return (
    <div className="flex items-center gap-2.5 text-sm text-[var(--editor-muted)]">
      <CheckCircle2 className={cn("size-4", done ? "text-[var(--editor-teal)]" : "text-[var(--editor-border-strong)]")} />
      {label}
    </div>
  )
}
