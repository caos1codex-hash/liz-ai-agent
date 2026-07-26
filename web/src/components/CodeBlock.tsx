// CodeBlock — bloque de código con syntax highlighting + botón copiar.
// Lazy-loaded: react-syntax-highlighter es pesado (795KB), solo se carga cuando
// el primer mensaje contiene un code block.

import { Prism as SyntaxHighlighter } from 'react-syntax-highlighter'
import { oneDark } from 'react-syntax-highlighter/dist/esm/styles/prism'
import { useState } from 'react'
import { cn } from '@/lib/utils'

interface CodeBlockProps {
  language: string | undefined
  value: string
}

export function CodeBlock({ language, value }: CodeBlockProps) {
  const [copied, setCopied] = useState(false)

  const handleCopy = async () => {
    try {
      await navigator.clipboard.writeText(value)
      setCopied(true)
      setTimeout(() => setCopied(false), 1800)
    } catch {
      // Fallback silencioso (permisos del portapapeles)
    }
  }

  const lang = (language ?? 'text').toLowerCase()
  const displayName = lang === 'text' || lang === 'plaintext' ? 'txt' : lang

  return (
    <div className="group relative my-3 overflow-hidden rounded-lg border border-border dark:border-border-dark">
      <div className="flex items-center justify-between border-b border-border bg-surface-muted px-3 py-1.5 dark:border-border-dark dark:bg-surface-dark-muted">
        <span className="font-mono text-xs text-text-muted dark:text-text-dark-muted">
          {displayName}
        </span>
        <button
          onClick={handleCopy}
          className={cn(
            'rounded px-2 py-0.5 text-xs font-medium transition-colors',
            copied
              ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-300'
              : 'text-text-muted hover:bg-surface hover:text-text dark:text-text-dark-muted dark:hover:bg-surface-dark-subtle dark:hover:text-text-dark',
          )}
          aria-label={copied ? 'Copiado' : 'Copiar código'}
        >
          {copied ? '✓ Copiado' : 'Copiar'}
        </button>
      </div>
      <SyntaxHighlighter
        language={lang}
        style={oneDark}
        customStyle={{
          margin: 0,
          padding: '0.875rem 1rem',
          fontSize: '0.8125rem',
          background: 'transparent',
        }}
        codeTagProps={{
          style: { fontFamily: 'JetBrains Mono, Menlo, Monaco, monospace' },
        }}
        wrapLongLines={false}
      >
        {value}
      </SyntaxHighlighter>
    </div>
  )
}

// Default export para permitir React.lazy() en Markdown.tsx
export default CodeBlock
