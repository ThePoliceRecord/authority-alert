function getSizeByNumber(maxNum, start = 0, gap = 1) {
  const sizeObj = {};
  for (let index = start; index <= maxNum; index += gap) {
    sizeObj[index] = index;
  }
  return sizeObj;
}

export default {
  content: ["./index.html", "./src/**/*.{js,ts,jsx,tsx}"],
  theme: {
    extend: {
      colors: {
        // Brand Primary Colors (from TPR.css)
        primary: {
          DEFAULT: "#2328bb", // blue-1 - main brand color
          hover: "#0065a3",   // blue-2 - hover state
          light: "#1fa9ff",   // cobalt-blue - light accent
          dark: "#1a237e",    // darker variant
        },
        // Secondary/Accent Colors
        secondary: {
          DEFAULT: "#ada9bb", // gray-1
          hover: "#797586",   // gray-2
          dark: "#404756",    // gray-3
        },
        // CTA/Success Colors
        cta: {
          DEFAULT: "#9be564", // SGBus Green
          hover: "#8ad454",
        },
        // Warning/Highlight Colors
        highlight: {
          DEFAULT: "#f3b61f", // Xanthous
          accent: "#f1d302",  // School Bus Yellow
        },
        // Error/Danger Colors
        error: {
          DEFAULT: "#e3170a", // Chili Red
          dark: "#730001",    // red-2
          light: "#a4000b",   // red-3
          burgundy: "#af2c2f", // red-1
        },
        // Neutral Colors
        surface: {
          DEFAULT: "#fdfffc", // Baby Powder - main surface
          dark: "#2a2a27",    // jet-gray - dark surface
          muted: "#e0e0e0",   // platinum
        },
        // Text Colors
        text: {
          DEFAULT: "#2a2a27", // jet-gray - primary text
          secondary: "#5e747f", // Payne's Gray
          muted: "#7f7f76",   // bs-gray
          light: "#e0e0e0",   // platinum - light text on dark bg
        },
        // Background Colors
        background: {
          DEFAULT: "#1f1f1b", // dark background
          light: "#f1f3f5",   // light background
          card: "#fdfffc",    // card background
          overlay: "rgba(31, 31, 27, 0.8)", // translucent dark
        },
        // Legacy colors for compatibility
        "3d": "#3d3d3d",
        disable: "#D2D9C3",
        selected: "#0344ff20", // translucent blue selection
        // Additional brand colors
        cobalt: "#1fa9ff",
        folly: "#ff1053",
        walnut: "#5f5449",
        platinum: "#e0e0e0",
        jet: "#2a2a27",
      },
      // Typography
      fontFamily: {
        sans: ['Inter', 'system-ui', '-apple-system', 'BlinkMacSystemFont', 'Segoe UI', 'Roboto', 'sans-serif'],
        display: ['Inter', 'system-ui', 'sans-serif'],
      },
      // Shadows for cards and elevated surfaces
      boxShadow: {
        'card': '0 4px 6px rgba(0, 0, 0, 0.1)',
        'card-hover': '0 6px 12px rgba(0, 0, 0, 0.15)',
        'elevated': '0 8px 16px rgba(0, 0, 0, 0.2)',
        'translucent': '2px 2px 4px 4px rgba(3, 68, 255, 0.4), -2px -2px 4px 4px rgba(3, 68, 255, 0.2)',
        'blue-glow': 'inset 5px 10px 10px 5px rgba(35, 40, 187, 0.3)',
      },
      // Border radius
      borderRadius: {
        ...getSizeByNumber(25),
        'card': '8px',
        'button': '6px',
        'input': '4px',
      },
      // Spacing
      width: getSizeByNumber(750),
      minWidth: getSizeByNumber(300),
      height: getSizeByNumber(100),
      fontSize: getSizeByNumber(50, 12),
      opacity: getSizeByNumber(1, 0, 0.5),
      zIndex: getSizeByNumber(100, 0, 10),
      spacing: getSizeByNumber(200, -60),
      // Transitions
      transitionDuration: {
        'fast': '150ms',
        'normal': '250ms',
        'slow': '350ms',
      },
    },
  },
  plugins: [],
};
