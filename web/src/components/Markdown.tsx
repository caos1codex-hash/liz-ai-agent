// Markdown — renderer con soporte GFM (tablas, listas, strikethrough) y code blocks.
// Componente memoizado para evitar re-renders innecesarios durante streaming.
// CodeBlock se carga via React.lazy para reducir el bundle inicial.

import { memo, Suspense, lazy } from 'react'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import { cn } from '@/lib/utils'

// Lazy load: react-syntax-highlighter es ~800KB, solo se carga cuando hay code blocks.
const CodeBlock = lazy(() => import('./CodeBlock'))

interface MarkdownProps {
  content: string
  className?: string
}

// Fallback simple mientras carga el CodeBlock (primer render).
function CodeFallback({ language }: { language?: string }) {
  return (
    <div className="my-3 rounded-lg border border-border bg-surface-muted px-3 py-2 text-xs text-text-muted dark:border-border-dark dark:bg-surface-dark-muted dark:text-text-dark-muted">
      <span className="font-mono">{language ?? 'txt'}</span> · cargando highlighter…
    </div>
  )
}

export const Markdown = memo(function Markdown({ content, className }: MarkdownProps) {
  return (
    <div
      className={cn(
        'prose prose-sm dark:prose-invert max-w-none',
        // Tipografía ajustada al tema Liz
        'prose-headings:font-semibold prose-headings:text-text dark:prose-headings:text-text-dark',
        'prose-p:text-text prose-p:leading-relaxed dark:prose-p:text-text-dark',
        'prose-strong:text-text dark:prose-strong:text-text-dark',
        'prose-a:text-liz-600 prose-a:no-underline hover:prose-a:underline dark:prose-a:text-liz-400',
        'prose-blockquote:border-liz-400 prose-blockquote:not-italic',
        'prose-code:rounded prose-code:bg-surface-muted prose-code:px-1 prose-code:py-0.5 prose-code:text-[0.85em] prose-code:font-mono prose-code:before:content-none prose-code:after:content-none',
        'dark:prose-code:bg-surface-dark-muted',
        'prose-pre:bg-transparent prose-pre:p-0 prose-pre:m-0',
        'prose-table:text-sm',
        'prose-th:border-border prose-th:bg-surface-muted prose-th:px-3 prose-th:py-1.5 prose-th:text-left',
        'dark:prose-th:border-border-dark dark:prose-th:bg-surface-dark-muted',
        'prose-td:border-border prose-td:px-3 prose-td:py-1.5',
        'dark:prose-td:border-border-dark',
        'prose-hr:border-border dark:prose-hr:border-border-dark',
        'prose-li:text-text dark:prose-li:text-text-dark',
        className,
      )}
    >
      <ReactMarkdown
        remarkPlugins={[remarkGfm]}
        components={{
          // CodeBlock personalizado con syntax highlighting + botón copiar.
          code({ className: codeClassName, children, ...props }) {
            const inline = !codeClassName
            const match = /language-(\w+)/.exec(codeClassName ?? '')
            const value = String(children ?? '').replace(/\n$/, '')

            if (inline) {
              return (
                <code className={codeClassName} {...props}>
                  {children}
                </code>
              )
            }
            return (
              <Suspense fallback={<CodeFallback language={match?.[1]} />}>
                <CodeBlock language={match?.[1]} value={value} />
              </Suspense>
            )
          },
          // Pre limpio (el CodeBlock ya tiene su propio contenedor).
          pre({ children }) {
            return <>{children}</>
          },
          // Links abren en nueva pestaña.
          a({ href, children }) {
            return (
              <a href={href} target="_blank" rel="noopener noreferrer">
                {children}
              </a>
            )
          },
        }}
      >
        {content}
      </ReactMarkdown>
    </div>
  )
})
