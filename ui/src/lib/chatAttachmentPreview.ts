export type AttachmentPreviewKind = 'image' | 'pdf' | 'markdown' | 'code' | 'text' | 'unsupported'

export type AttachmentPreviewDescriptor = {
  kind: AttachmentPreviewKind
  language?: string
}

const codeLanguageByExtension: Record<string, string> = {
  '.ts': 'typescript',
  '.tsx': 'tsx',
  '.js': 'javascript',
  '.jsx': 'jsx',
  '.mjs': 'javascript',
  '.cjs': 'javascript',
  '.json': 'json',
  '.yaml': 'yaml',
  '.yml': 'yaml',
  '.go': 'go',
  '.py': 'python',
  '.sh': 'bash',
  '.bash': 'bash',
  '.zsh': 'bash',
  '.sql': 'sql',
  '.html': 'markup',
  '.htm': 'markup',
  '.css': 'css',
  '.scss': 'scss',
  '.less': 'less',
  '.xml': 'markup',
  '.toml': 'toml',
  '.ini': 'ini',
  '.java': 'java',
  '.rb': 'ruby',
  '.rs': 'rust',
  '.c': 'c',
  '.h': 'c',
  '.cpp': 'cpp',
  '.hpp': 'cpp',
  '.cc': 'cpp',
  '.cs': 'csharp',
  '.php': 'php',
  '.swift': 'swift',
  '.kt': 'kotlin',
  '.kts': 'kotlin',
  '.dockerfile': 'docker',
  '.env': 'bash',
}

const markdownExtensions = new Set(['.md', '.markdown', '.mdx'])

function normalizeMimeType(mimeType?: string): string {
  return (mimeType ?? '').split(';')[0]?.trim().toLowerCase() ?? ''
}

function normalizedExtension(fileName: string): string {
  const value = fileName.trim().toLowerCase()
  if (value === 'dockerfile') return '.dockerfile'
  if (value.endsWith('.env')) return '.env'
  const dot = value.lastIndexOf('.')
  return dot >= 0 ? value.slice(dot) : ''
}

export function describeAttachmentPreview(
  fileName: string,
  mimeType?: string,
  attachmentKind?: string,
): AttachmentPreviewDescriptor {
  const ext = normalizedExtension(fileName)
  const mime = normalizeMimeType(mimeType)
  const normalizedKind = (attachmentKind ?? '').trim().toLowerCase()

  if (normalizedKind === 'image' || mime.startsWith('image/')) {
    return { kind: 'image' }
  }
  if (normalizedKind === 'pdf' || mime === 'application/pdf' || ext === '.pdf') {
    return { kind: 'pdf' }
  }
  if (markdownExtensions.has(ext)) {
    return { kind: 'markdown' }
  }
  const language = codeLanguageByExtension[ext]
  if (language) {
    return { kind: 'code', language }
  }
  if (normalizedKind === 'text' || mime.startsWith('text/')) {
    return { kind: 'text' }
  }
  return { kind: 'unsupported' }
}

export function canPreviewAttachmentTarget(
  fileName: string,
  mimeType?: string,
  attachmentKind?: string,
): boolean {
  return describeAttachmentPreview(fileName, mimeType, attachmentKind).kind !== 'unsupported'
}
