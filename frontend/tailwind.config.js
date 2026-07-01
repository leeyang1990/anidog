/** @type {import('tailwindcss').Config} */
module.exports = {
  content: ["./index.html", "./src/**/*.{vue,js,ts,jsx,tsx}"],
  darkMode: 'class',
  theme: {
    extend: {
      fontFamily: {
        sans: [
          '"ZCOOL KuaiLe"',
          'Nunito',
          '"PingFang SC"',
          '"Hiragino Sans GB"',
          '"Microsoft YaHei"',
          '-apple-system',
          'BlinkMacSystemFont',
          'sans-serif',
        ],
        display: [
          '"ZCOOL KuaiLe"',
          'Nunito',
          '"PingFang SC"',
          'sans-serif',
        ],
        mono: [
          'ui-monospace',
          'SFMono-Regular',
          'Menlo',
          'Monaco',
          'Consolas',
          'monospace',
        ],
      },
      colors: {
        // shadcn 兼容色（业务代码继续用，只是值变了）
        border: 'hsl(var(--border) / <alpha-value>)',
        input: 'hsl(var(--input) / <alpha-value>)',
        ring: 'hsl(var(--ring) / <alpha-value>)',
        background: 'hsl(var(--background) / <alpha-value>)',
        foreground: 'hsl(var(--foreground) / <alpha-value>)',
        primary: {
          DEFAULT: 'hsl(var(--primary) / <alpha-value>)',
          foreground: 'hsl(var(--primary-foreground) / <alpha-value>)',
        },
        secondary: {
          DEFAULT: 'hsl(var(--secondary) / <alpha-value>)',
          foreground: 'hsl(var(--secondary-foreground) / <alpha-value>)',
        },
        destructive: {
          DEFAULT: 'hsl(var(--destructive) / <alpha-value>)',
          foreground: 'hsl(var(--destructive-foreground) / <alpha-value>)',
        },
        muted: {
          DEFAULT: 'hsl(var(--muted) / <alpha-value>)',
          foreground: 'hsl(var(--muted-foreground) / <alpha-value>)',
        },
        accent: {
          DEFAULT: 'hsl(var(--accent) / <alpha-value>)',
          foreground: 'hsl(var(--accent-foreground) / <alpha-value>)',
        },
        popover: {
          DEFAULT: 'hsl(var(--popover) / <alpha-value>)',
          foreground: 'hsl(var(--popover-foreground) / <alpha-value>)',
        },
        card: {
          DEFAULT: 'hsl(var(--card) / <alpha-value>)',
          foreground: 'hsl(var(--card-foreground) / <alpha-value>)',
        },
        sidebar: {
          DEFAULT: 'hsl(var(--sidebar-background) / <alpha-value>)',
          foreground: 'hsl(var(--sidebar-foreground) / <alpha-value>)',
          primary: 'hsl(var(--sidebar-primary) / <alpha-value>)',
          'primary-foreground': 'hsl(var(--sidebar-primary-foreground) / <alpha-value>)',
          accent: 'hsl(var(--sidebar-accent) / <alpha-value>)',
          'accent-foreground': 'hsl(var(--sidebar-accent-foreground) / <alpha-value>)',
          border: 'hsl(var(--sidebar-border) / <alpha-value>)',
          ring: 'hsl(var(--sidebar-ring) / <alpha-value>)',
          'muted-foreground': 'hsl(var(--sidebar-muted-foreground) / <alpha-value>)',
        },
        chart: {
          '1': 'hsl(var(--chart-1) / <alpha-value>)',
          '2': 'hsl(var(--chart-2) / <alpha-value>)',
          '3': 'hsl(var(--chart-3) / <alpha-value>)',
          '4': 'hsl(var(--chart-4) / <alpha-value>)',
          '5': 'hsl(var(--chart-5) / <alpha-value>)',
        },
        // 动森命名色（直接 bg-ac-grass / text-ac-earth 等）
        ac: {
          cream:       '#F7F4E9',
          milk:        '#FFFDF7',
          grass:       '#7CB342',
          'grass-dark':'#558B2F',
          'grass-light':'#AED581',
          sun:         '#FFB74D',
          'sun-dark':  '#F57C00',
          sky:         '#81D4FA',
          'sky-dark':  '#0288D1',
          heart:       '#E57373',
          'heart-dark':'#C62828',
          leaf:        '#66BB6A',
          'leaf-dark': '#388E3C',
          wood:        '#8D6E63',
          'wood-dark': '#5D4037',
          earth:       '#5D4037',
          sand:        '#E8DCC4',
          'sand-dark': '#C9B98F',
          night:       '#3E2723',
        },
      },
      borderRadius: {
        lg: 'var(--radius)',
        md: 'calc(var(--radius) - 4px)',
        sm: 'calc(var(--radius) - 8px)',
        xl: 'calc(var(--radius) + 6px)',
        '2xl': 'calc(var(--radius) + 12px)',
        '3xl': 'calc(var(--radius) + 18px)',
      },
      boxShadow: {
        // 动森软棕阴影（基色 rgba(141,110,99,...) 而非纯黑）
        sm: '0 2px 4px rgba(141, 110, 99, 0.10)',
        DEFAULT: '0 4px 10px rgba(141, 110, 99, 0.12)',
        md: '0 4px 12px rgba(141, 110, 99, 0.14)',
        lg: '0 10px 28px rgba(141, 110, 99, 0.18)',
        xl: '0 16px 40px rgba(141, 110, 99, 0.22)',
        '2xl': '0 24px 60px rgba(141, 110, 99, 0.26)',
        ac: '0 6px 16px rgba(141, 110, 99, 0.16)',
        // 动森按钮"凸起"专用：底部 4px 实色边
        'ac-button': 'inset 0 -4px 0 0 rgba(85, 139, 47, 1)',
        'ac-button-pressed': 'inset 0 -1px 0 0 rgba(85, 139, 47, 1)',
        'ac-button-sun': 'inset 0 -4px 0 0 rgba(245, 124, 0, 1)',
        'ac-button-heart': 'inset 0 -4px 0 0 rgba(198, 40, 40, 1)',
        'ac-button-secondary': 'inset 0 -4px 0 0 rgba(201, 185, 143, 1)',
        none: 'none',
      },
      letterSpacing: {
        tighter: '-0.04em',
        tight: '-0.02em',
        normal: '0',
        wide: '0.02em',
        wider: '0.04em',
        widest: '0.08em',
      },
      keyframes: {
        'fade-in': {
          from: { opacity: '0', transform: 'translateY(8px)' },
          to: { opacity: '1', transform: 'translateY(0)' },
        },
        'fade-out': {
          from: { opacity: '1', transform: 'translateY(0)' },
          to: { opacity: '0', transform: 'translateY(-4px)' },
        },
        'bounce-soft': {
          '0%, 100%': { transform: 'translateY(0)' },
          '50%': { transform: 'translateY(-3px)' },
        },
        'wiggle': {
          '0%, 100%': { transform: 'rotate(-2deg)' },
          '50%': { transform: 'rotate(2deg)' },
        },
        'pop-in': {
          '0%': { opacity: '0', transform: 'scale(0.94)' },
          '70%': { opacity: '1', transform: 'scale(1.02)' },
          '100%': { opacity: '1', transform: 'scale(1)' },
        },
        'slide-in-right': {
          from: { transform: 'translateX(100%)' },
          to: { transform: 'translateX(0)' },
        },
        'spin-leaf': {
          from: { transform: 'rotate(0deg)' },
          to: { transform: 'rotate(360deg)' },
        },
        'typewriter': {
          from: { width: '0' },
          to: { width: '100%' },
        },
        'caret-blink': {
          '0%, 100%': { borderColor: 'transparent' },
          '50%': { borderColor: 'currentColor' },
        },
      },
      animation: {
        'fade-in': 'fade-in 0.3s cubic-bezier(0.4, 0, 0.2, 1)',
        'fade-out': 'fade-out 0.2s ease-out',
        'bounce-soft': 'bounce-soft 1.4s ease-in-out infinite',
        'wiggle': 'wiggle 0.5s ease-in-out',
        'pop-in': 'pop-in 0.32s cubic-bezier(0.34, 1.56, 0.64, 1)',
        'slide-in-right': 'slide-in-right 0.28s cubic-bezier(0.32, 0.72, 0, 1)',
        'spin-leaf': 'spin-leaf 1.4s linear infinite',
      },
      transitionTimingFunction: {
        'ac': 'cubic-bezier(0.34, 1.56, 0.64, 1)', // 动森式回弹
      },
    },
  },
  plugins: [],
}
