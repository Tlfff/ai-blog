import { Analytics } from '@vercel/analytics/next'
import type { Metadata, Viewport } from 'next'
import { AuthProvider } from '@/hooks/use-auth'
import { SITE_IMAGES } from '@/lib/site-images'
import './globals.css'

const geistSans = { variable: '--font-geist-sans' }
const geistMono = { variable: '--font-geist-mono' }

const themeScript = `
(() => {
  try {
    const saved = localStorage.getItem('site-theme');
    const dark = saved ? saved === 'dark' : window.matchMedia('(prefers-color-scheme: dark)').matches;
    document.documentElement.classList.toggle('dark', dark);
    document.documentElement.dataset.theme = dark ? 'dark' : 'light';
  } catch (_) {}
})();`

export const metadata: Metadata = {
  title: 'Link start！· 睦子米的个人博客',
  description: '睦子米的个人博客：记录技术、生活与正在发生的灵感。',
  generator: 'v0.app',
  icons: {
    icon: [
      { url: SITE_IMAGES.brand.icon, type: 'image/svg+xml' },
      { url: SITE_IMAGES.brand.iconLight, media: '(prefers-color-scheme: dark)' },
      { url: SITE_IMAGES.brand.iconDark, media: '(prefers-color-scheme: light)' },
    ],
    apple: SITE_IMAGES.brand.appleIcon,
  },
}

export const viewport: Viewport = {
  colorScheme: 'light',
  themeColor: [
    { media: '(prefers-color-scheme: light)', color: '#f8f1e8' },
  ],
}

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode
}>) {
  return (
    <html lang="zh-CN" suppressHydrationWarning className={`bg-background ${geistSans.variable} ${geistMono.variable}`} data-scroll-behavior="smooth">
      <head>
        <script dangerouslySetInnerHTML={{ __html: themeScript }} />
      </head>
      <body className="font-sans antialiased">
        <AuthProvider>{children}</AuthProvider>
        {process.env.NODE_ENV === 'production' && <Analytics />}
      </body>
    </html>
  )
}
