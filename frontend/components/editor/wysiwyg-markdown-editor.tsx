"use client"

import {
  forwardRef,
  useEffect,
  useImperativeHandle,
  useMemo,
  useRef,
  useState,
  type KeyboardEvent as ReactKeyboardEvent,
  type MutableRefObject,
} from "react"
import { createPortal } from "react-dom"
import {
  Bold,
  Braces,
  Code2,
  Heading1,
  Heading2,
  Heading3,
  ImageIcon,
  Italic,
  List,
  ListOrdered,
  Loader2,
  Minus,
  Quote,
  Redo2,
  RotateCcw,
  Strikethrough,
  Trash2,
  Undo2,
} from "lucide-react"
import { EditorContent, NodeViewWrapper, ReactNodeViewRenderer, useEditor, type NodeViewProps } from "@tiptap/react"
import { BubbleMenu } from "@tiptap/react/menus"
import StarterKit from "@tiptap/starter-kit"
import Image from "@tiptap/extension-image"
import Placeholder from "@tiptap/extension-placeholder"
import { Markdown } from "@tiptap/markdown"
import { cn } from "@/lib/utils"

export interface WysiwygImagePreview {
  previewUrl: string
  fileName: string
  status: "uploading" | "success" | "failed" | "saved"
  error?: string
}

export interface PreparedEditorImage {
  source: string
  previewUrl: string
  fileName: string
  alt: string
}

export interface WysiwygMarkdownEditorHandle {
  insertImage: (file: File) => void
  focus: () => void
}

interface WysiwygMarkdownEditorProps {
  value: string
  disabled?: boolean
  imagePreviews: Map<string, WysiwygImagePreview>
  onChange: (markdown: string) => void
  onPrepareImage: (file: File) => PreparedEditorImage | null
  onRemoveImage: (source: string) => void
  onRetryImage: (source: string) => void
}

interface SlashMenuState {
  query: string
  start: number
  left: number
  top: number
  selectedIndex: number
}

interface ImageCallbacks {
  onRemoveImage: (source: string) => void
  onRetryImage: (source: string) => void
}

const STATUS_LABELS = {
  uploading: "上传中",
  success: "已上传，等待保存",
  failed: "上传失败",
  saved: "已绑定",
} as const

const SLASH_COMMANDS = [
  { id: "paragraph", label: "正文", hint: "普通文本", icon: Braces, keywords: "paragraph text 正文" },
  { id: "heading-1", label: "一级标题", hint: "# + 空格", icon: Heading1, keywords: "heading h1 标题" },
  { id: "heading-2", label: "二级标题", hint: "## + 空格", icon: Heading2, keywords: "heading h2 标题" },
  { id: "heading-3", label: "三级标题", hint: "### + 空格", icon: Heading3, keywords: "heading h3 标题" },
  { id: "bullet-list", label: "无序列表", hint: "- + 空格", icon: List, keywords: "bullet list 列表" },
  { id: "ordered-list", label: "有序列表", hint: "1. + 空格", icon: ListOrdered, keywords: "ordered list 列表" },
  { id: "quote", label: "引用", hint: "> + 空格", icon: Quote, keywords: "blockquote quote 引用" },
  { id: "code-block", label: "代码块", hint: "``` + 空格", icon: Code2, keywords: "code block 代码" },
  { id: "horizontal-rule", label: "分割线", hint: "---", icon: Minus, keywords: "divider rule 分割线" },
] as const

type SlashCommandId = (typeof SLASH_COMMANDS)[number]["id"]

function createManagedImageExtension(
  previewsRef: MutableRefObject<Map<string, WysiwygImagePreview>>,
  callbacksRef: MutableRefObject<ImageCallbacks>,
) {
  return Image.extend({
    addAttributes() {
      return {
        ...this.parent?.(),
        source: {
          default: null,
          parseHTML: (element) => element.getAttribute("data-source"),
          renderHTML: (attributes) =>
            attributes.source ? { "data-source": attributes.source } : {},
        },
        fileName: { default: null },
        status: { default: "saved" },
        error: { default: null },
      }
    },
    parseMarkdown(token, helpers) {
      const source = token.href
      const preview = previewsRef.current.get(source)
      return helpers.createNode("image", {
        src: preview?.previewUrl ?? source,
        source,
        alt: token.text,
        title: token.title,
        fileName: preview?.fileName ?? token.text ?? "文章图片",
        status: preview?.status ?? "saved",
        error: preview?.error ?? null,
      })
    },
    renderMarkdown(node) {
      const source = node.attrs?.source ?? node.attrs?.src ?? ""
      const alt = node.attrs?.alt ?? node.attrs?.fileName ?? "文章图片"
      const title = node.attrs?.title ?? ""
      return title ? `![${alt}](${source} "${title}")` : `![${alt}](${source})`
    },
    addNodeView() {
      return ReactNodeViewRenderer((props) => (
        <ManagedImageNode
          {...props}
          callbacksRef={callbacksRef}
        />
      ))
    },
  }).configure({
    allowBase64: true,
    HTMLAttributes: { class: "wysiwyg-image" },
  })
}

function ManagedImageNode({
  node,
  deleteNode,
  selected,
  callbacksRef,
}: NodeViewProps & { callbacksRef: MutableRefObject<ImageCallbacks> }) {
  const source = node.attrs.source || node.attrs.src
  const status = (node.attrs.status || "saved") as keyof typeof STATUS_LABELS

  return (
    <NodeViewWrapper
      as="figure"
      className={cn("wysiwyg-image-node", selected && "is-selected")}
      data-drag-handle
    >
      {/* eslint-disable-next-line @next/next/no-img-element */}
      <img src={node.attrs.src} alt={node.attrs.alt || "文章图片"} draggable={false} />
      <figcaption>
        <span className="flex min-w-0 items-center gap-2">
          <ImageIcon className="size-3.5 shrink-0 text-[var(--editor-sky)]" />
          <span className="truncate">{node.attrs.fileName || node.attrs.alt || "文章图片"}</span>
          <span className={cn("editor-upload-status", status === "failed" && "is-error")}>
            {status === "uploading" && <Loader2 className="size-3 animate-spin" />}
            {STATUS_LABELS[status]}
          </span>
          {node.attrs.error ? <span className="truncate text-destructive">{node.attrs.error}</span> : null}
        </span>
        <span className="flex shrink-0 items-center gap-3">
          {status === "failed" ? (
            <button type="button" onClick={() => callbacksRef.current.onRetryImage(source)}>
              <RotateCcw className="size-3.5" />
              重试
            </button>
          ) : null}
          <button
            type="button"
            onClick={() => {
              callbacksRef.current.onRemoveImage(source)
              deleteNode()
            }}
          >
            <Trash2 className="size-3.5" />
            删除
          </button>
        </span>
      </figcaption>
    </NodeViewWrapper>
  )
}

export const WysiwygMarkdownEditor = forwardRef<
  WysiwygMarkdownEditorHandle,
  WysiwygMarkdownEditorProps
>(function WysiwygMarkdownEditor(
  {
    value,
    disabled = false,
    imagePreviews,
    onChange,
    onPrepareImage,
    onRemoveImage,
    onRetryImage,
  },
  ref,
) {
  const previewsRef = useRef(imagePreviews)
  const callbacksRef = useRef<ImageCallbacks>({ onRemoveImage, onRetryImage })
  const onChangeRef = useRef(onChange)
  const onPrepareImageRef = useRef(onPrepareImage)
  const lastEmittedMarkdownRef = useRef(value)
  const initializedRef = useRef(false)
  const applyingExternalContentRef = useRef(false)
  const [slashMenu, setSlashMenu] = useState<SlashMenuState | null>(null)
  const [mounted, setMounted] = useState(false)

  previewsRef.current = imagePreviews
  callbacksRef.current = { onRemoveImage, onRetryImage }
  onChangeRef.current = onChange
  onPrepareImageRef.current = onPrepareImage

  const managedImage = useMemo(
    () => createManagedImageExtension(previewsRef, callbacksRef),
    [],
  )

  const editor = useEditor({
    immediatelyRender: false,
    editable: !disabled,
    extensions: [
      StarterKit.configure({
        heading: { levels: [1, 2, 3] },
        codeBlock: { HTMLAttributes: { class: "editor-code-block" } },
      }),
      managedImage,
      Placeholder.configure({
        placeholder: "输入 / 打开命令，或用 Markdown 快捷语法开始写作…",
        includeChildren: true,
      }),
      Markdown.configure({ indentation: { style: "space", size: 2 } }),
    ],
    content: value || "",
    contentType: "markdown",
    editorProps: {
      attributes: {
        class: "wysiwyg-editor-content",
        spellcheck: "true",
      },
      handlePaste(view, event) {
        const imageItem = Array.from(event.clipboardData?.items ?? []).find((item) =>
          item.type.startsWith("image/"),
        )
        const file = imageItem?.getAsFile()
        if (!file) return false

        const prepared = onPrepareImageRef.current(file)
        if (!prepared) return true
        const imageNode = view.state.schema.nodes.image.create({
          src: prepared.previewUrl,
          source: prepared.source,
          alt: prepared.alt,
          fileName: prepared.fileName,
          status: "uploading",
        })
        view.dispatch(view.state.tr.replaceSelectionWith(imageNode).scrollIntoView())
        return true
      },
    },
    onUpdate({ editor: currentEditor }) {
      if (!initializedRef.current || applyingExternalContentRef.current) return
      const markdown = currentEditor.getMarkdown()
      lastEmittedMarkdownRef.current = markdown
      onChangeRef.current(markdown)
      updateSlashMenu(currentEditor)
    },
    onSelectionUpdate({ editor: currentEditor }) {
      updateSlashMenu(currentEditor)
    },
  })

  useEffect(() => setMounted(true), [])

  useEffect(() => {
    editor?.setEditable(!disabled)
  }, [disabled, editor])

  useEffect(() => {
    if (!editor) return
    if (initializedRef.current && value === lastEmittedMarkdownRef.current) return

    const currentMarkdown = editor.getMarkdown()
    if (currentMarkdown !== value) {
      const { from, to } = editor.state.selection
      const parsedContent = value && editor.markdown
        ? editor.markdown.parse(value)
        : ""
      applyingExternalContentRef.current = true
      editor.commands.setContent(parsedContent, { emitUpdate: false })
      const maxPosition = editor.state.doc.content.size
      editor.commands.setTextSelection({
        from: Math.min(from, maxPosition),
        to: Math.min(to, maxPosition),
      })
      applyingExternalContentRef.current = false
    }
    lastEmittedMarkdownRef.current = value
    initializedRef.current = true
  }, [editor, value])

  useEffect(() => {
    if (!editor) return

    const transaction = editor.state.tr
    let changed = false
    editor.state.doc.descendants((node, position) => {
      if (node.type.name !== "image") return
      const source = node.attrs.source || node.attrs.src
      const preview = imagePreviews.get(source)
      if (!preview) return

      const nextAttributes = {
        ...node.attrs,
        src: preview.previewUrl,
        source,
        fileName: preview.fileName,
        status: preview.status,
        error: preview.error ?? null,
      }
      if (
        node.attrs.src !== nextAttributes.src ||
        node.attrs.fileName !== nextAttributes.fileName ||
        node.attrs.status !== nextAttributes.status ||
        node.attrs.error !== nextAttributes.error
      ) {
        transaction.setNodeMarkup(position, undefined, nextAttributes)
        changed = true
      }
    })
    if (changed) editor.view.dispatch(transaction.setMeta("preventUpdate", true))
  }, [editor, imagePreviews])

  useImperativeHandle(ref, () => ({
    insertImage(file) {
      if (!editor) return
      const prepared = onPrepareImageRef.current(file)
      if (!prepared) return
      editor
        .chain()
        .focus()
        .insertContent([
          {
            type: "image",
            attrs: {
              src: prepared.previewUrl,
              source: prepared.source,
              alt: prepared.alt,
              fileName: prepared.fileName,
              status: "uploading",
            },
          },
          { type: "paragraph" },
        ])
        .run()
    },
    focus() {
      editor?.chain().focus().run()
    },
  }), [editor])

  function updateSlashMenu(currentEditor: NonNullable<typeof editor>) {
    const { selection } = currentEditor.state
    if (!selection.empty || !selection.$from.parent.isTextblock) {
      setSlashMenu(null)
      return
    }

    const textBefore = selection.$from.parent.textBetween(0, selection.$from.parentOffset, " ", " ")
    const match = /(?:^|\s)\/([^\s/]*)$/.exec(textBefore)
    if (!match) {
      setSlashMenu(null)
      return
    }

    const query = match[1].toLowerCase()
    const coords = currentEditor.view.coordsAtPos(selection.from)
    const start = selection.from - query.length - 1
    setSlashMenu((current) => ({
      query,
      start,
      left: Math.min(coords.left, window.innerWidth - 288),
      top: Math.min(coords.bottom + 8, window.innerHeight - 380),
      selectedIndex: current?.query === query ? current.selectedIndex : 0,
    }))
  }

  const filteredCommands = SLASH_COMMANDS.filter((command) =>
    `${command.label} ${command.keywords}`.toLowerCase().includes(slashMenu?.query ?? ""),
  )

  function executeSlashCommand(commandId: SlashCommandId) {
    if (!editor || !slashMenu) return
    const chain = editor.chain().focus().deleteRange({
      from: slashMenu.start,
      to: editor.state.selection.from,
    })

    switch (commandId) {
      case "paragraph":
        chain.setParagraph().run()
        break
      case "heading-1":
        chain.setHeading({ level: 1 }).run()
        break
      case "heading-2":
        chain.setHeading({ level: 2 }).run()
        break
      case "heading-3":
        chain.setHeading({ level: 3 }).run()
        break
      case "bullet-list":
        chain.toggleBulletList().run()
        break
      case "ordered-list":
        chain.toggleOrderedList().run()
        break
      case "quote":
        chain.toggleBlockquote().run()
        break
      case "code-block":
        chain.setCodeBlock().run()
        break
      case "horizontal-rule":
        chain.setHorizontalRule().run()
        break
    }
    setSlashMenu(null)
  }

  function handleKeyDown(event: ReactKeyboardEvent<HTMLDivElement>) {
    if (!slashMenu || filteredCommands.length === 0) return

    if (event.key === "ArrowDown" || event.key === "ArrowUp") {
      event.preventDefault()
      const direction = event.key === "ArrowDown" ? 1 : -1
      setSlashMenu((current) =>
        current
          ? {
              ...current,
              selectedIndex:
                (current.selectedIndex + direction + filteredCommands.length) % filteredCommands.length,
            }
          : null,
      )
      return
    }

    if (event.key === "Enter") {
      event.preventDefault()
      executeSlashCommand(filteredCommands[slashMenu.selectedIndex]?.id ?? filteredCommands[0].id)
      return
    }

    if (event.key === "Escape") {
      event.preventDefault()
      setSlashMenu(null)
    }
  }

  if (!editor) {
    return <div className="min-h-80 animate-pulse rounded-xl bg-[var(--editor-surface-muted)]" />
  }

  return (
    <div className="wysiwyg-editor" onKeyDownCapture={handleKeyDown}>
      <BubbleMenu editor={editor} options={{ placement: "top" }}>
        <div className="wysiwyg-bubble-menu">
          <ToolbarButton
            label="加粗"
            active={editor.isActive("bold")}
            onClick={() => editor.chain().focus().toggleBold().run()}
          >
            <Bold className="size-4" />
          </ToolbarButton>
          <ToolbarButton
            label="斜体"
            active={editor.isActive("italic")}
            onClick={() => editor.chain().focus().toggleItalic().run()}
          >
            <Italic className="size-4" />
          </ToolbarButton>
          <ToolbarButton
            label="删除线"
            active={editor.isActive("strike")}
            onClick={() => editor.chain().focus().toggleStrike().run()}
          >
            <Strikethrough className="size-4" />
          </ToolbarButton>
          <ToolbarButton
            label="行内代码"
            active={editor.isActive("code")}
            onClick={() => editor.chain().focus().toggleCode().run()}
          >
            <Code2 className="size-4" />
          </ToolbarButton>
        </div>
      </BubbleMenu>

      <div className="wysiwyg-editor-toolbar">
        <div className="flex items-center gap-1">
          <ToolbarButton label="撤销" onClick={() => editor.chain().focus().undo().run()}>
            <Undo2 className="size-4" />
          </ToolbarButton>
          <ToolbarButton label="重做" onClick={() => editor.chain().focus().redo().run()}>
            <Redo2 className="size-4" />
          </ToolbarButton>
          <span className="mx-1 h-5 w-px bg-[var(--editor-border)]" />
          <ToolbarButton
            label="二级标题"
            active={editor.isActive("heading", { level: 2 })}
            onClick={() => editor.chain().focus().toggleHeading({ level: 2 }).run()}
          >
            <Heading2 className="size-4" />
          </ToolbarButton>
          <ToolbarButton
            label="无序列表"
            active={editor.isActive("bulletList")}
            onClick={() => editor.chain().focus().toggleBulletList().run()}
          >
            <List className="size-4" />
          </ToolbarButton>
          <ToolbarButton
            label="引用"
            active={editor.isActive("blockquote")}
            onClick={() => editor.chain().focus().toggleBlockquote().run()}
          >
            <Quote className="size-4" />
          </ToolbarButton>
          <ToolbarButton
            label="代码块"
            active={editor.isActive("codeBlock")}
            onClick={() => editor.chain().focus().toggleCodeBlock().run()}
          >
            <Code2 className="size-4" />
          </ToolbarButton>
        </div>
        <span className="text-xs text-[var(--editor-muted)]">输入 / 使用命令</span>
      </div>

      <EditorContent editor={editor} />

      {mounted && slashMenu && filteredCommands.length > 0
        ? createPortal(
            <div
              className="wysiwyg-slash-menu"
              style={{ left: slashMenu.left, top: slashMenu.top }}
            >
              <p className="px-3 pb-2 pt-2 text-[11px] font-medium tracking-[0.16em] text-[var(--editor-muted)]">
                插入内容块
              </p>
              {filteredCommands.map((command, index) => {
                const Icon = command.icon
                return (
                  <button
                    key={command.id}
                    type="button"
                    className={cn(index === slashMenu.selectedIndex && "is-selected")}
                    onMouseDown={(event) => event.preventDefault()}
                    onClick={() => executeSlashCommand(command.id)}
                  >
                    <span className="grid size-8 place-items-center rounded-lg bg-[var(--editor-surface-muted)] text-[var(--editor-sky)]">
                      <Icon className="size-4" />
                    </span>
                    <span className="min-w-0 flex-1 text-left">
                      <strong>{command.label}</strong>
                      <small>{command.hint}</small>
                    </span>
                  </button>
                )
              })}
            </div>,
            document.body,
          )
        : null}
    </div>
  )
})

function ToolbarButton({
  children,
  label,
  active = false,
  onClick,
}: {
  children: React.ReactNode
  label: string
  active?: boolean
  onClick: () => void
}) {
  return (
    <button
      type="button"
      title={label}
      aria-label={label}
      aria-pressed={active}
      onMouseDown={(event) => event.preventDefault()}
      onClick={onClick}
      className={cn("wysiwyg-toolbar-button", active && "is-active")}
    >
      {children}
    </button>
  )
}
