pkgname=wizado
pkgver=0.2.0
pkgrel=1
pkgdesc="Steam gaming mode for Hyprland (Omarchy)"
arch=('any')
url="https://github.com/REPLACE_ME/wizado"
license=('MIT')
depends=('bash' 'python')
optdepends=(
  'steam: Steam client'
  'gamescope: Gaming compositor'
  'gamemode: System optimizations'
  'mangohud: Performance overlay'
  'hyprland: Hyprland compositor'
)
install="${pkgname}.install"

# For local development, use current directory
# For AUR, replace with proper source URL
source=()
sha256sums=()

package() {
  cd "${startdir}"

  # Install scripts
  install -d "${pkgdir}/usr/share/${pkgname}"
  cp -a scripts "${pkgdir}/usr/share/${pkgname}/"

  # Install CLI
  install -Dm755 "bin/wizado" "${pkgdir}/usr/bin/wizado"
}
