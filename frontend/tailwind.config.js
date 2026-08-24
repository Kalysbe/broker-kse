/** @type {import('tailwindcss').Config} */
export default {
  content: ["./index.html", "./src/**/*.{ts,tsx}"],
  theme: {
    extend: {
      colors: {
        brand: {
          DEFAULT: "#3AA8B5",
          dark: "#2A8A96",
          light: "#7DD3DC",
        },
        navy: {
          DEFAULT: "#0B2429",
          mid: "#12343B",
          soft: "#1A4A53",
        },
        ink: "#12262B",
        muted: "#5B6B70",
        line: "#D7E2E4",
        paper: "#F3F7F8",
      },
      boxShadow: {
        card: "0 1px 2px rgba(11, 36, 41, 0.04), 0 12px 28px -14px rgba(11, 36, 41, 0.18)",
        glow: "0 0 0 1px rgba(58, 168, 181, 0.18), 0 18px 40px -18px rgba(58, 168, 181, 0.45)",
      },
      backgroundImage: {
        mesh: "radial-gradient(1200px 600px at -10% -20%, rgba(58,168,181,0.35), transparent 55%), radial-gradient(900px 500px at 110% 10%, rgba(18,52,59,0.9), transparent 50%), linear-gradient(160deg, #07181c 0%, #0b2429 45%, #12343b 100%)",
      },
    },
  },
  plugins: [],
};
