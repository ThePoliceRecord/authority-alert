# Brand Assets Directory

This directory contains locally vendored brand assets for the Supervisor UI.

## Purpose

All assets referenced by the redesigned UI (images, icons, fonts) must be included here to ensure the UI works fully **offline** without requiring external CDN dependencies.

## Asset Categories

### Images
Place brand images here (logos, backgrounds, icons, etc.)

### Fonts (if needed)
Place any custom fonts here if required by the brand guidelines.

## Usage

Assets in this directory are copied as-is by Vite during build and are accessible via stable URL paths like `/brand/filename.ext`.

## External References to Localize

From TPR.css, the following external URLs were identified:
- `https://cdn.thepolicerecord.com/static/images/media/blueback.jpeg`
- `https://dev.thepolicerecord.com/static/images/media/adoptback.jpeg`

If these images are needed, download them and place them here, then update CSS references to use local paths.

## Notes

- TPR.css in `solutions/supervisor/TPR.css` is **reference only** and **must not ship** in compiled output
- All assets must be bundled into the built `dist/` output
- No runtime fetching of brand assets from the internet
