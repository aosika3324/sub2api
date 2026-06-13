/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{vue,js,ts,jsx,tsx}'],
  darkMode: 'class',
  theme: {
    extend: {
      colors: {
        // Product neutral palette. Keep `primary` API-compatible while moving
        // the interface away from teal gradients and template-like glow.
        primary: {
          50: '#f7f7f5',
          100: '#eceae6',
          200: '#dad6cf',
          300: '#bdb7ad',
          400: '#928b81',
          500: '#5f5a52',
          600: '#3f3b36',
          700: '#2e2b27',
          800: '#211f1c',
          900: '#171613',
          950: '#0d0c0a'
        },
        accent: {
          50: '#fbf1ed',
          100: '#f7e0d6',
          200: '#efc0ac',
          300: '#e5a07f',
          400: '#e08a6b',
          500: '#c96442',
          600: '#b25237',
          700: '#94422c',
          800: '#773726',
          900: '#622f22',
          950: '#361712'
        },
        dark: {
          50: '#f8f8f7',
          100: '#efeeec',
          200: '#ddd9d4',
          300: '#bdb7ad',
          400: '#8f887e',
          500: '#6c665f',
          600: '#524d47',
          700: '#3d3934',
          800: '#2b2925',
          900: '#1d1b18',
          950: '#11100e'
        },
        // Override Tailwind's default COOL (blue-tinted) neutral palettes with a
        // warm neutral aligned to the Claude coral/clay theme. Hundreds of
        // components use bg-gray-*/text-gray-*/border-gray-* (and slate-*) for
        // surfaces, text and borders; without this they read cold against the
        // warm --ui-* tokens (e.g. the slate-blue table header).
        gray: {
          50: '#faf9f5',
          100: '#f2ede3',
          200: '#e5ddcf',
          300: '#d4cbbb',
          400: '#a8a094',
          500: '#78716a',
          600: '#57514a',
          700: '#403b35',
          800: '#2a2622',
          900: '#1c1a16',
          950: '#131110'
        },
        slate: {
          50: '#faf9f5',
          100: '#f2ede3',
          200: '#e5ddcf',
          300: '#d4cbbb',
          400: '#a8a094',
          500: '#78716a',
          600: '#57514a',
          700: '#403b35',
          800: '#2a2622',
          900: '#1c1a16',
          950: '#131110'
        }
      },
      fontFamily: {
        sans: [
          'system-ui',
          '-apple-system',
          'BlinkMacSystemFont',
          'Segoe UI',
          'Roboto',
          'Helvetica Neue',
          'Arial',
          'PingFang SC',
          'Hiragino Sans GB',
          'Microsoft YaHei',
          'sans-serif'
        ],
        mono: ['ui-monospace', 'SFMono-Regular', 'Menlo', 'Monaco', 'Consolas', 'monospace'],
        serif: ['Fraunces', 'Georgia', '"Songti SC"', '"STSong"', 'serif']
      },
      boxShadow: {
        glass: '0 1px 2px rgba(17, 16, 14, 0.04)',
        'glass-sm': '0 1px 2px rgba(17, 16, 14, 0.04)',
        glow: '0 1px 2px rgba(17, 16, 14, 0.04)',
        'glow-lg': '0 1px 2px rgba(17, 16, 14, 0.04)',
        card: '0 1px 2px rgba(17, 16, 14, 0.04)',
        'card-hover': '0 8px 24px rgba(17, 16, 14, 0.06)',
        'inner-glow': 'inset 0 1px 0 rgba(255, 255, 255, 0.1)'
      },
      backgroundImage: {
        'gradient-radial': 'radial-gradient(var(--tw-gradient-stops))',
        'gradient-primary': 'linear-gradient(135deg, #171613 0%, #2e2b27 100%)',
        'gradient-dark': 'linear-gradient(135deg, #1d1b18 0%, #11100e 100%)',
        'gradient-glass':
          'linear-gradient(135deg, rgba(255,255,255,0.1) 0%, rgba(255,255,255,0.05) 100%)',
        'mesh-gradient': 'linear-gradient(180deg, rgba(255,255,255,0) 0%, rgba(255,255,255,0) 100%)'
      },
      animation: {
        'fade-in': 'fadeIn 0.3s ease-out',
        'slide-up': 'slideUp 0.3s ease-out',
        'slide-down': 'slideDown 0.3s ease-out',
        'slide-in-right': 'slideInRight 0.3s ease-out',
        'scale-in': 'scaleIn 0.2s ease-out',
        'pulse-slow': 'pulse 3s cubic-bezier(0.4, 0, 0.6, 1) infinite',
        shimmer: 'shimmer 2s linear infinite',
        glow: 'glow 2s ease-in-out infinite alternate'
      },
      keyframes: {
        fadeIn: {
          '0%': { opacity: '0' },
          '100%': { opacity: '1' }
        },
        slideUp: {
          '0%': { opacity: '0', transform: 'translateY(10px)' },
          '100%': { opacity: '1', transform: 'translateY(0)' }
        },
        slideDown: {
          '0%': { opacity: '0', transform: 'translateY(-10px)' },
          '100%': { opacity: '1', transform: 'translateY(0)' }
        },
        slideInRight: {
          '0%': { opacity: '0', transform: 'translateX(20px)' },
          '100%': { opacity: '1', transform: 'translateX(0)' }
        },
        scaleIn: {
          '0%': { opacity: '0', transform: 'scale(0.95)' },
          '100%': { opacity: '1', transform: 'scale(1)' }
        },
        shimmer: {
          '0%': { backgroundPosition: '-200% 0' },
          '100%': { backgroundPosition: '200% 0' }
        },
        glow: {
          '0%': { boxShadow: '0 0 20px rgba(201, 100, 66, 0.25)' },
          '100%': { boxShadow: '0 0 30px rgba(201, 100, 66, 0.4)' }
        }
      },
      backdropBlur: {
        xs: '2px'
      },
      borderRadius: {
        '4xl': '2rem'
      }
    }
  },
  plugins: []
}
