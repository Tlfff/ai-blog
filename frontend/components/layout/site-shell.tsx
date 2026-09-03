import type { ReactNode } from "react"
import { SiteHeader } from "./site-header"
import { Curtain } from "./curtain"

export function SiteShell({
  children,
  immersiveHeader = false,
}: {
  children: ReactNode
  immersiveHeader?: boolean
}) {
  return (
    <div className="flex min-h-screen flex-col bg-paper">
      <Curtain />
      <div className="paper-grain" aria-hidden />
      <SiteHeader overlay={immersiveHeader} />
      <main className={immersiveHeader ? "relative z-10 flex-1" : "relative z-10 flex-1 pt-16"}>
        {children}
      </main>
    </div>
  )
}
