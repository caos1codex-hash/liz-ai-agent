/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{ts,tsx}'],
  darkMode: 'class',
  theme: {
    extend: {
      colors: {
        // Paleta Liz — morado primario con acentos
        liz: {
          50: '#f5f3ff',
          100: '#ede9fe',
          200: '#ddd6fe',
          300: '#c4b5fd',
          400: '#a78bfa',
          500: '#8b5cf6',
          600: '#7c3aed',
          700: '#6d28d9',
          800: '#5b21b6',
          900: '#4c1d95',
          950: '#2e1065',
        },
        // Tokens semánticos del tema (se sobreescriben por dark:)
        surface: {
          DEFAULT: '#ffffff',
          subtle: '#f9fafb',
          muted: '#f3f4f6',
          dark: '#0f0f12',
          'dark-subtle': '#16161a',
          'dark-muted': '#1f1f24',
        },
        text: {
          DEFAULT: '#111827',
          muted: '#6b7280',
          subtle: '#9ca3af',
          dark: '#f3f4f6',
          'dark-muted': '#9ca3af',
          'dark-subtle': '#6b7280',
        },
        border: {
          DEFAULT: '#e5e7eb',
          muted: '#f3f4f6',
          dark: '#2a2a30',
          'dark-muted': '#1f1f24',
        },
      },
      fontFamily: {
        sans: ['Inter', 'system-ui', '-apple-system', 'sans-serif'],
        mono: ['JetBrains Mono', 'Menlo', 'Monaco', 'monospace'],
      },
      animation: {
        'fade-in': 'fadeIn 200ms ease-out',
        'slide-up': 'slideUp 250ms ease-out',
        'pulse-soft': 'pulseSoft 1.6s ease-in-out infinite',
        'blink': 'blink 1s step-end infinite',
      },
      keyframes: {
        fadeIn: {
          '0%': { opacity: '0' },
          '100%': { opacity: '1' },
        },
        slideUp: {
          '0%': { opacity: '0', transform: 'translateY(8px)' },
          '100%': { opacity: '1', transform: 'translateY(0)' },
        },
        pulseSoft: {
          '0%, 100%': { opacity: '1' },
          '50%': { opacity: '0.5' },
        },
        blink: {
          '0%, 100%': { opacity: '1' },
          '50%': { opacity: '0' },
        },
      },
    },
  },
  plugins: [],
}
