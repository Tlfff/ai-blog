import { readFile, readdir } from "node:fs/promises"
import path from "node:path"
import { IMAGE_DIRECTORIES, type HeroSlide } from "@/lib/site-images"

const IMAGE_EXTENSIONS = new Set([".avif", ".gif", ".jpeg", ".jpg", ".png", ".svg", ".webp"])
const PUBLIC_ROOT = path.join(process.cwd(), "public")

interface CarouselMetadata {
  alt?: string
  objectPosition?: string
}

async function listImageUrls(publicDirectory: string): Promise<string[]> {
  const relativeDirectory = publicDirectory.replace(/^\//, "")
  const directory = path.join(PUBLIC_ROOT, relativeDirectory)
  const entries = await readdir(directory, { withFileTypes: true })

  return entries
    .filter((entry) => entry.isFile() && IMAGE_EXTENSIONS.has(path.extname(entry.name).toLowerCase()))
    .map((entry) => `${publicDirectory}/${entry.name}`)
    .sort((left, right) => left.localeCompare(right, "zh-CN", { numeric: true }))
}

async function readCarouselMetadata(): Promise<Record<string, CarouselMetadata>> {
  const metadataPath = path.join(
    PUBLIC_ROOT,
    IMAGE_DIRECTORIES.homeCarousel.replace(/^\//, ""),
    "metadata.json",
  )

  try {
    return JSON.parse(await readFile(metadataPath, "utf8")) as Record<string, CarouselMetadata>
  } catch {
    return {}
  }
}

function filenameToAlt(src: string): string {
  const filename = path.basename(src, path.extname(src)).replace(/^\d+[-_]?/, "")
  return filename.replace(/[-_]+/g, " ")
}

export async function getHomeImageCatalog(): Promise<{
  carouselSlides: HeroSlide[]
  articleFallbackCovers: string[]
}> {
  const [carouselImages, articleFallbackCovers, metadata] = await Promise.all([
    listImageUrls(IMAGE_DIRECTORIES.homeCarousel),
    listImageUrls(IMAGE_DIRECTORIES.articleCovers),
    readCarouselMetadata(),
  ])

  return {
    carouselSlides: carouselImages.map((src) => {
      const filename = path.basename(src)
      return {
        src,
        alt: metadata[filename]?.alt || filenameToAlt(src),
        objectPosition: metadata[filename]?.objectPosition || "center center",
      }
    }),
    articleFallbackCovers,
  }
}
