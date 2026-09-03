"use client"

import { useEffect, useState, useCallback } from "react"
import { AnimatePresence, motion, useReducedMotion } from "motion/react"
import {
  SITE_INTRO_COMPLETE_DATASET_KEY,
  SITE_INTRO_COMPLETE_EVENT,
} from "./site-intro"

const INTRO_TITLE = Array.from("Link start！")
const EASE_IN = [0.16, 1, 0.3, 1] as const
const EASE_OUT = [0.7, 0, 0.84, 0] as const
let introPlayedThisVisit = false

export function Curtain() {
  const reduceMotion = useReducedMotion()
  const [open, setOpen] = useState(introPlayedThisVisit)

  const dismiss = useCallback(() => {
    introPlayedThisVisit = true
    document.documentElement.dataset[SITE_INTRO_COMPLETE_DATASET_KEY] = "true"
    window.dispatchEvent(new Event(SITE_INTRO_COMPLETE_EVENT))
    setOpen(true)
  }, [])

  useEffect(() => {
    if (introPlayedThisVisit) {
      dismiss()
      return
    }

    if (reduceMotion) {
      dismiss()
      return
    }

    const timer = window.setTimeout(dismiss, 2200)
    const onKey = (event: KeyboardEvent) => {
      if (event.key === "Tab") return
      dismiss()
    }
    window.addEventListener("keydown", onKey)

    return () => {
      window.clearTimeout(timer)
      window.removeEventListener("keydown", onKey)
    }
  }, [dismiss, reduceMotion])

  useEffect(() => {
    if (reduceMotion || open) return
    const { style } = document.body
    const previous = style.overflow
    style.overflow = "hidden"

    return () => {
      style.overflow = previous
    }
  }, [open, reduceMotion])

  if (reduceMotion) return null

  return (
    <AnimatePresence>
      {!open && (
        <motion.div
          className="fixed inset-0 z-[100] cursor-pointer"
          onClick={dismiss}
          exit={{ opacity: 1 }}
          aria-label="跳过开幕动画并进入站点"
          role="button"
          tabIndex={0}
        >
          {/* 上下对开的米白幕布 */}
          <motion.div
            className="absolute inset-x-0 top-0 h-[calc(50%+1px)] overflow-hidden bg-paper"
            exit={{ y: "-100%" }}
            transition={{ duration: 0.62, ease: EASE_OUT, delay: 0.12 }}
          >
            <div className="absolute inset-x-0 bottom-0 top-0 flex items-end justify-center">
              <CurtainInner half="top" />
            </div>
          </motion.div>

          <motion.div
            className="absolute inset-x-0 bottom-0 h-[calc(50%+1px)] overflow-hidden bg-paper"
            exit={{ y: "100%" }}
            transition={{ duration: 0.62, ease: EASE_OUT, delay: 0.12 }}
          >
            <div className="absolute inset-x-0 bottom-0 top-0 flex items-start justify-center">
              <CurtainInner half="bottom" />
            </div>
          </motion.div>

          {/* 第二拍：粉色色块横扫 */}
          <motion.div
            className="pointer-events-none absolute inset-0 origin-left bg-sakura"
            initial={{ scaleX: 0 }}
            animate={{ scaleX: [0, 1, 1, 0] }}
            transition={{
              duration: 1.15,
              times: [0, 0.34, 0.62, 1],
              ease: EASE_IN,
              delay: 0.55,
            }}
            style={{ transformOrigin: "left center" }}
          />

          <motion.div
            className="pointer-events-none absolute inset-x-0 bottom-10 flex justify-center"
            exit={{ opacity: 0 }}
            transition={{ duration: 0.12 }}
          >
            <motion.span
              className="label-meta"
              initial={{ opacity: 0 }}
              animate={{ opacity: 1 }}
              transition={{ delay: 1.7, duration: 0.3 }}
            >
              click / any key to skip
            </motion.span>
          </motion.div>
        </motion.div>
      )}
    </AnimatePresence>
  )
}

/**
 * 幕布内容在上下两半各渲染一次，靠 translateY 对齐成一个整体，
 * 这样对开时文字会被幕布自然裁成两半带走。
 */
function CurtainInner({ half }: { half: "top" | "bottom" }) {
  return (
    <div
      className="relative flex h-[100vh] w-full flex-col items-center justify-center gap-5"
      style={{ transform: half === "top" ? "translateY(50%)" : "translateY(-50%)" }}
    >
      {/* 网点底 */}
      <motion.div
        className="texture-halftone absolute inset-0 opacity-0"
        animate={{ opacity: 0.16 }}
        transition={{ delay: 1.5, duration: 0.5 }}
      />

      <div className="relative flex flex-wrap justify-center gap-x-[0.15em]">
        {INTRO_TITLE.map((char, index) => (
          <motion.span
            key={`${char}-${index}`}
            className="title-display text-3xl text-ink md:text-5xl"
            initial={{ opacity: 0, y: 10 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ delay: 0.1 + index * 0.075, duration: 0.32, ease: EASE_IN }}
          >
            {char}
          </motion.span>
        ))}
      </div>

      <div className="relative overflow-hidden">
        <motion.div
          className="label-meta text-ink"
          initial={{ y: "110%" }}
          animate={{ y: 0 }}
          transition={{ delay: 1.42, duration: 0.44, ease: EASE_IN }}
        >
          personal blog &nbsp;/&nbsp; tech &amp; life
        </motion.div>
      </div>

      <motion.div
        className="relative h-px bg-ink"
        initial={{ width: 0 }}
        animate={{ width: 180 }}
        transition={{ delay: 1.6, duration: 0.5, ease: EASE_IN }}
      />
    </div>
  )
}
