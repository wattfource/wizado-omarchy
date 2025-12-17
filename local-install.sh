#!/usr/bin/env bash
set -euo pipefail

# local-install-test.sh
# Simulates an AUR install by building the package locally using makepkg and installing it.

echo ">>> Cleaning up previous builds..."
rm -rf pkg src *.pkg.tar.zst *.pkg.tar.xz

echo ">>> Creating source tarball..."
# PKGBUILD expects a specific source format. For local testing, we'll trick it
# or just copy the files. To properly simulate, we should archive the current dir.
# However, usually for dev builds we use a -git PKGBUILD or just point to local files.
# Let's adjust PKGBUILD temporarily or create a 'release' tarball.

VERSION=$(grep '^pkgver=' packaging/PKGBUILD | cut -d= -f2)
TARBALL="wizado-${VERSION}.tar.gz"

# Create a temporary directory for the "upstream" source
mkdir -p build_tmp/wizado-${VERSION}
cp -r bin scripts packaging LICENSE README.md build_tmp/wizado-${VERSION}/ 2>/dev/null || true
# Also copy missing files if any (like bin/wizado which is tracked)

# Create the tarball
tar -C build_tmp -czf "$TARBALL" "wizado-${VERSION}"
rm -rf build_tmp

echo ">>> Created $TARBALL"

# Move PKGBUILD to current dir for makepkg (standard practice is running in a clean dir)
cp packaging/PKGBUILD .
cp packaging/wizado.install .

# Update PKGBUILD to use our local tarball
sed -i "s|source=.*|source=(\"${TARBALL}\")|g" PKGBUILD
sed -i "s|sha256sums=.*|sha256sums=('SKIP')|g" PKGBUILD

echo ">>> Building package..."
makepkg -f

echo ">>> Installing package..."
# Find the built package
PKG_FILE=$(ls -1 wizado-*.pkg.tar.zst 2>/dev/null | head -n1)

if [[ -z "$PKG_FILE" ]]; then
  echo "ERROR: Package build failed, no .pkg.tar.zst found."
  exit 1
fi

sudo pacman -U "$PKG_FILE"

echo ">>> Installation complete!"
echo "You can now run 'wizado setup' from anywhere."

# Cleanup
rm -f PKGBUILD wizado.install "$TARBALL"
rm -rf pkg src *.pkg.tar.zst *.pkg.tar.xz

