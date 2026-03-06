import { Modal, ModalBody, ModalContent, ModalHeader } from '@heroui/react'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import { Prism as SyntaxHighlighter } from 'react-syntax-highlighter'
import { oneDark, oneLight } from 'react-syntax-highlighter/dist/esm/styles/prism'

import { useThemeStore } from '@/stores/themeStore'

import type { AttachmentPreviewKind } from '@/lib/chatAttachmentPreview'

export type AttachmentPreviewState = {
  name: string
  kind: AttachmentPreviewKind
  url?: string
  content?: string
  language?: string
  loading?: boolean
  error?: string
  revokeOnClose: boolean
}

type AttachmentPreviewModalProps = {
  preview: AttachmentPreviewState | null
  onClose: () => void
}

function renderCodeBlock(content: string, language: string | undefined, dark: boolean) {
  if (!language) {
    return (
      <pre className="overflow-auto rounded-lg border bg-background p-4 text-sm whitespace-pre-wrap">
        {content}
      </pre>
    )
  }
  return (
    <SyntaxHighlighter
      language={language}
      style={dark ? oneDark : oneLight}
      showLineNumbers
      wrapLongLines
      customStyle={{ margin: 0, borderRadius: '0.75rem', fontSize: '0.875rem' }}
    >
      {content}
    </SyntaxHighlighter>
  )
}

export function AttachmentPreviewModal({ preview, onClose }: AttachmentPreviewModalProps) {
  const theme = useThemeStore((s) => s.theme)
  const dark = theme === 'dark'

  return (
    <Modal
      isOpen={Boolean(preview)}
      onOpenChange={(open) => {
        if (!open) onClose()
      }}
      size="5xl"
      scrollBehavior="inside"
    >
      <ModalContent>
        {() => (
          <>
            <ModalHeader>{preview?.name ?? '附件预览'}</ModalHeader>
            <ModalBody className="min-h-[60vh]">
              {!preview ? null : preview.loading ? (
                <div className="flex h-[60vh] items-center justify-center text-sm text-muted-foreground">
                  正在加载附件预览…
                </div>
              ) : preview.error ? (
                <div className="flex h-[60vh] items-center justify-center text-sm text-danger">
                  {preview.error}
                </div>
              ) : preview.kind === 'image' && preview.url ? (
                <div className="flex h-full items-center justify-center overflow-auto">
                  <img
                    src={preview.url}
                    alt={preview.name}
                    className="max-h-[70vh] max-w-full rounded-lg object-contain"
                  />
                </div>
              ) : preview.kind === 'pdf' && preview.url ? (
                <iframe
                  title={preview.name}
                  src={preview.url}
                  className="min-h-[70vh] w-full rounded-lg border"
                />
              ) : preview.kind === 'markdown' ? (
                <div className="chat-markdown rounded-lg border bg-background/40 p-4 text-sm">
                  <ReactMarkdown
                    remarkPlugins={[remarkGfm]}
                    components={{
                      code(props) {
                        const { children, className } = props
                        const match = /language-(\w+)/.exec(className || '')
                        const value = String(children).replace(/\n$/, '')
                        return match ? (
                          <SyntaxHighlighter
                            PreTag="div"
                            language={match[1]}
                            style={dark ? oneDark : oneLight}
                            showLineNumbers
                            wrapLongLines
                            customStyle={{ margin: 0, borderRadius: '0.75rem', fontSize: '0.875rem' }}
                          >
                            {value}
                          </SyntaxHighlighter>
                        ) : (
                          <code className={className}>{children}</code>
                        )
                      },
                    }}
                  >
                    {preview.content ?? ''}
                  </ReactMarkdown>
                </div>
              ) : preview.kind === 'code' ? (
                renderCodeBlock(preview.content ?? '', preview.language, dark)
              ) : preview.kind === 'text' ? (
                <pre className="overflow-auto rounded-lg border bg-background p-4 text-sm whitespace-pre-wrap">
                  {preview.content ?? ''}
                </pre>
              ) : (
                <div className="flex h-[60vh] items-center justify-center text-sm text-muted-foreground">
                  当前附件类型暂不支持预览
                </div>
              )}
            </ModalBody>
          </>
        )}
      </ModalContent>
    </Modal>
  )
}
