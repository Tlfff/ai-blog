export const MAX_IMAGE_UPLOAD_BYTES = 10 * 1024 * 1024

export function getImageSizeError(file: Pick<File, "size">, subject = "图片"): string | null {
  if (file.size <= 0) return `${subject}文件不能为空`
  if (file.size > MAX_IMAGE_UPLOAD_BYTES) return `${subject}大小不能超过10MB`
  return null
}

export function assertImageSize(file: Pick<File, "size">, subject = "图片"): void {
  const error = getImageSizeError(file, subject)
  if (error) throw new Error(error)
}
