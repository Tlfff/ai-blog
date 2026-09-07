const imagePath = (path: string) => `/images/${path}`

export interface HeroSlide {
  src: string
  alt: string
  objectPosition: string
}

export const IMAGE_DIRECTORIES = {
  homeCarousel: "/images/home/carousel",
  articleCovers: "/images/articles/covers",
  avatars: "/images/avatars",
  pages: "/images/pages",
  brand: "/images/brand",
  placeholders: "/images/placeholders",
} as const

export const SITE_IMAGES = {
  articles: {
    detailFallbackCover: imagePath("articles/defaults/bocchi-lace.jpg"),
    mockCovers: {
      go: imagePath("articles/covers/sq-1.jpg"),
      react: imagePath("articles/covers/sq-2.jpg"),
      typescript: imagePath("articles/covers/sq-3.jpg"),
      database: imagePath("articles/covers/yln-1.jpg"),
      frontend: imagePath("articles/covers/sq-1.jpg"),
      distributedSystems: imagePath("articles/covers/sq-2.jpg"),
      career: imagePath("articles/covers/sq-3.jpg"),
    },
  },
  avatars: {
    authorFallback: imagePath("avatars/bocchi-sunglasses.jpg"),
    admin: imagePath("avatars/admin.png"),
    chen: imagePath("avatars/user-chen.png"),
    lin: imagePath("avatars/user-lin.png"),
    su: imagePath("avatars/user-su.png"),
    ze: imagePath("avatars/user-ze.png"),
  },
  pages: {
    primarySky: imagePath("pages/bq-1.png"),
    secondarySky: imagePath("pages/bq-2.png"),
    aboutHero: imagePath("pages/bq-3.png"),
    registerHero: imagePath("pages/bq-5.png"),
  },
  brand: {
    icon: imagePath("brand/icon.svg"),
    iconLight: imagePath("brand/icon-light-32x32.png"),
    iconDark: imagePath("brand/icon-dark-32x32.png"),
    appleIcon: imagePath("brand/apple-icon.png"),
  },
  placeholders: {
    generic: imagePath("placeholders/placeholder.svg"),
    user: imagePath("placeholders/placeholder-user.jpg"),
    logo: imagePath("placeholders/placeholder-logo.svg"),
  },
} as const
