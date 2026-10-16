/** @type {import('tailwindcss').Config} */
export default {
  content: [
    "./index.html",
    "./src/**/*.{js,ts,jsx,tsx}",
  ],
  theme: {
    extend: {
      colors: {
        background: '#121214', // Deep warm graphite
        surface: {
          DEFAULT: '#18181c', // Warm dark card surface
          elevated: '#202026', // Slightly elevated surface
          highlight: '#282832', // Hover / active surface
        },
        border: {
          subtle: '#26262e', // Hairline neutral border
          focus: '#3f3f4e',
        },
        stone: {
          50: '#fafaf9',
          100: '#f5f5f4',
          200: '#e7e5e4',
          300: '#d6d3d1',
          400: '#a8a29e',
          500: '#78716c',
          600: '#57534e',
          700: '#44403c',
          800: '#292524',
          900: '#1c1917',
          950: '#0c0a09',
        },
        sage: {
          DEFAULT: '#84a98c', // Muted organic green (ICMP Active)
          hover: '#9bbba2',
          subtle: 'rgba(132, 169, 140, 0.12)',
        },
        terracotta: {
          DEFAULT: '#d97706', // Warm amber / terracotta (ARP Firewall)
          hover: '#f59e0b',
          subtle: 'rgba(217, 119, 6, 0.12)',
        },
        clay: {
          DEFAULT: '#a3a3a3', // Neutral slate for private MAC
          subtle: 'rgba(163, 163, 163, 0.1)',
        },
      },
      fontFamily: {
        sans: ['Inter', 'system-ui', '-apple-system', 'BlinkMacSystemFont', 'Segoe UI', 'Roboto', 'sans-serif'],
        mono: ['JetBrains Mono', 'ui-monospace', 'SFMono-Regular', 'Menlo', 'Monaco', 'monospace'],
      },
    },
  },
  plugins: [],
}
