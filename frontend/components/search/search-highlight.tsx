interface SearchHighlightProps {
  value: string
  className?: string
}

/**
 * 仅把后端搜索高亮协议中的 <em> 片段渲染为 mark，避免直接注入 HTML。
 */
export function SearchHighlight({ value, className }: SearchHighlightProps) {
  const parts = value.split(/(<em>[\s\S]*?<\/em>)/gi)

  return (
    <span className={className}>
      {parts.map((part, index) => {
        const match = part.match(/^<em>([\s\S]*?)<\/em>$/i)
        if (!match) return <span key={index}>{part}</span>

        return (
          <mark
            key={index}
            className="rounded-sm bg-sakura-wash px-0.5 text-inherit"
          >
            {match[1]}
          </mark>
        )
      })}
    </span>
  )
}
