#!/usr/bin/env bash
# Download all vendored front-end assets and regenerate the Tailwind CSS.
# Run this once after cloning, and again whenever templates change (for Tailwind).
#
# Requirements: curl, tar (standard on Linux/macOS)
set -euo pipefail

KATEX_VERSION="0.16.11"
HTMX_VERSION="2.0.4"
MARKED_VERSION="14"
TAILWIND_VERSION="3.4.17"

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
VENDOR="$ROOT/static/vendor"
TOOLS="$ROOT/.tools"

mkdir -p "$VENDOR/katex/fonts" "$VENDOR/fonts" "$TOOLS"

echo "→ Downloading HTMX $HTMX_VERSION..."
curl -sL -o "$VENDOR/htmx.min.js" \
  "https://unpkg.com/htmx.org@${HTMX_VERSION}/dist/htmx.min.js"

echo "→ Downloading marked $MARKED_VERSION..."
curl -sL -o "$VENDOR/marked.min.js" \
  "https://cdn.jsdelivr.net/npm/marked@${MARKED_VERSION}/marked.min.js"

echo "→ Downloading KaTeX $KATEX_VERSION..."
TMP=$(mktemp -d)
curl -sL "https://registry.npmjs.org/katex/-/katex-${KATEX_VERSION}.tgz" \
  | tar -xz -C "$TMP"
cp "$TMP/package/dist/katex.min.css"              "$VENDOR/katex/"
cp "$TMP/package/dist/katex.min.js"               "$VENDOR/katex/"
cp "$TMP/package/dist/contrib/auto-render.min.js" "$VENDOR/katex/"
cp "$TMP/package/dist/fonts/"*.woff2              "$VENDOR/katex/fonts/"
rm -rf "$TMP"

echo "→ Downloading Inter font (latin + extended subsets)..."
declare -A INTER_FILES=(
  [inter-cyrillic-ext]=UcC73FwrK3iLTeHuS_nVMrMxCp50SjIa2JL7SUc
  [inter-cyrillic]=UcC73FwrK3iLTeHuS_nVMrMxCp50SjIa0ZL7SUc
  [inter-greek-ext]=UcC73FwrK3iLTeHuS_nVMrMxCp50SjIa2ZL7SUc
  [inter-greek]=UcC73FwrK3iLTeHuS_nVMrMxCp50SjIa1pL7SUc
  [inter-vietnamese]=UcC73FwrK3iLTeHuS_nVMrMxCp50SjIa2pL7SUc
  [inter-latin-ext]=UcC73FwrK3iLTeHuS_nVMrMxCp50SjIa25L7SUc
  [inter-latin]=UcC73FwrK3iLTeHuS_nVMrMxCp50SjIa1ZL7
)
for name in "${!INTER_FILES[@]}"; do
  hash="${INTER_FILES[$name]}"
  curl -sL -o "$VENDOR/fonts/${name}.woff2" \
    "https://fonts.gstatic.com/s/inter/v20/${hash}.woff2" &
done
wait

echo "→ Downloading Tailwind CLI v$TAILWIND_VERSION..."
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"
case "$ARCH" in
  x86_64) ARCH="x64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac
curl -sL -o "$TOOLS/tailwindcss" \
  "https://github.com/tailwindlabs/tailwindcss/releases/download/v${TAILWIND_VERSION}/tailwindcss-${OS}-${ARCH}"
chmod +x "$TOOLS/tailwindcss"

echo "→ Generating Tailwind CSS..."
cd "$ROOT"
"$TOOLS/tailwindcss" -c tailwind.config.js -i static/input.css -o static/vendor/tailwind.css --minify

echo "✓ All assets ready."
