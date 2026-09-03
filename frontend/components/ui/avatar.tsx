import { cn } from "@/lib/utils"

interface AvatarProps {
  src?: string
  alt: string
  size?: number
  className?: string
}

export function Avatar({ src, alt, size = 40, className }: AvatarProps) {
  const fallback = alt.slice(0, 1).toUpperCase()
  return (
    <span
      className={cn(
        "relative inline-flex shrink-0 items-center justify-center overflow-hidden rounded-full bg-gradient-to-br from-primary/20 to-accent/20 text-sm font-medium text-primary-foreground",
        className,
      )}
      style={{ width: size, height: size }}
    >
      {src ? (
        // eslint-disable-next-line @next/next/no-img-element
        <img src={src || "/placeholder.svg"} alt={alt} width={size} height={size} className="h-full w-full object-cover" />
      ) : (
        <span>{fallback}</span>
      )}
      <span className="absolute inset-0 rounded-full border border-primary/30 pointer-events-none" />
    </span>
  )
}