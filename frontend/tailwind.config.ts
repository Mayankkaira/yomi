import type { Config } from 'tailwindcss';

// Yomi design tokens.
// Palette roles:
//   canvas   — app background (warm paper)
//   surface  — cards, panels, tables
//   ink      — text hierarchy (primary / secondary / muted / faint)
//   line     — hairline borders and dividers
//   primary  — trust green: actions, active states, "verified" semantics
//   amber    — pending / attention
//   red      — rejected / destructive
//   slate    — revoked / expired / neutral-neutral
//   blue     — informational links and focus
export default {
  darkMode: 'class',
  content: ['./index.html', './src/**/*.{ts,tsx}'],
  theme: {
    extend: {
      colors: {
        // Surface/text tokens are CSS variables so dark mode can flip them.
        canvas: 'var(--canvas)',
        surface: 'var(--surface)',
        ink: {
          DEFAULT: 'var(--ink)',
          secondary: 'var(--ink-secondary)',
          muted: 'var(--ink-muted)',
          faint: 'var(--ink-faint)',
        },
        line: 'var(--line)',
        lineStrong: 'var(--line-strong)',
        // Subtle neutral fill for hovers, active rows, muted surfaces.
        tint: 'var(--tint)',
        primary: {
          50: '#EDF6F1',
          100: '#D8EBE0',
          200: '#B0D7C2',
          300: '#82BDA0',
          400: '#4E9E7B',
          500: '#21835E',
          600: '#12694A',
          700: '#0E543C',
          800: '#0B4230',
          900: '#083427',
        },
        amber: {
          50: '#FDF6EA',
          100: '#FAEBCB',
          500: '#B7791F',
          600: '#96600F',
          700: '#7A4C0B',
        },
        red: {
          50: '#FCF0EE',
          100: '#F8DCD8',
          500: '#C2422F',
          600: '#A43424',
          700: '#872A1E',
        },
        slate: {
          50: '#F2F2F1',
          100: '#E3E3E1',
          500: '#6E716F',
          600: '#575A58',
          700: '#444744',
        },
        blue: {
          50: '#EEF4FB',
          500: '#3B6FB6',
          600: '#2E5A94',
        },
      },
      fontFamily: {
        sans: ['Inter', 'system-ui', '-apple-system', 'Segoe UI', 'Roboto', 'sans-serif'],
        mono: ['"IBM Plex Mono"', 'ui-monospace', 'SFMono-Regular', 'Menlo', 'Consolas', 'monospace'],
      },
      fontSize: {
        '2xs': ['0.6875rem', { lineHeight: '1rem' }],
        display: ['1.75rem', { lineHeight: '2.125rem', letterSpacing: '-0.025em' }],
      },
      letterSpacing: {
        label: '0.08em',
        tight: '-0.02em',
      },
      boxShadow: {
        // Mercury-style flatness: hairline borders carry structure, shadows
        // are reserved for floating layers (modal, toast) only.
        card: 'none',
        pop: '0 12px 32px rgba(20,23,28,0.12)',
      },
      borderRadius: {
        panel: '0.5rem',
      },
    },
  },
  plugins: [],
} satisfies Config;
