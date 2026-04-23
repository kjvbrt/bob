/** @type {import('tailwindcss').Config} */
module.exports = {
  darkMode: 'class',
  content: ['./templates/**/*.html'],
  theme: {
    extend: {
      borderRadius: { '2xl': '1rem', '3xl': '1.5rem' },
      fontFamily: { sans: ['Inter', 'ui-sans-serif', 'system-ui'] }
    }
  }
}
