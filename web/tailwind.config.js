/** @type {import('tailwindcss').Config} */
export default {
  content: [
    "./index.html",
    "./src/**/*.{js,ts,jsx,tsx}",
  ],
  darkMode: 'class',
  theme: {
    extend: {
      colors: {
        bg: '#090d16',
        card: '#111827',
        border: '#1f2937',
        accent: '#3b82f6',
      },
    },
  },
  plugins: [],
}
