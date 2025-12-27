# Maintainer: Sean Fournier <sean@wattfource.com>
pkgname=wizado
pkgver=0.2.0
pkgrel=1
pkgdesc="Steam gaming mode for Hyprland (Omarchy)"
arch=('any')
url="https://github.com/wattfource/wizado-omarchy"
license=('MIT')
depends=('bash' 'curl' 'gum' 'jq')
optdepends=(
  'steam: Steam client'
  'gamescope: Gaming compositor'
  'gamemode: System optimizations'
  'mangohud: Performance overlay'
  'hyprland: Hyprland compositor'
)
install="${pkgname}.install"
source=("${pkgname}-${pkgver}.tar.gz::https://github.com/wattfource/wizado-omarchy/archive/refs/tags/v${pkgver}.tar.gz")
sha256sums=('6ef8817a550a983f8c8c2b0e46d99d982fc8195589e59a6959625e1da50bb1d2')

package() {
  cd "wizado-omarchy-${pkgver}"

  # Install scripts to /usr/share/wizado
  install -d "${pkgdir}/usr/share/${pkgname}/scripts"
  cp -a scripts/* "${pkgdir}/usr/share/${pkgname}/scripts/"

  # Install CLI wrapper
  install -Dm755 "bin/wizado" "${pkgdir}/usr/bin/wizado"

  # Install launchers to /usr/bin
  for launcher in scripts/launchers/*; do
    if [[ -f "$launcher" ]]; then
      install -Dm755 "$launcher" "${pkgdir}/usr/bin/$(basename "$launcher")"
    fi
  done

  # Install default config
  install -Dm644 "scripts/config/default.conf" "${pkgdir}/usr/share/${pkgname}/default.conf"

  # Install waybar config examples
  install -Dm644 "scripts/config/waybar-module.jsonc" "${pkgdir}/usr/share/${pkgname}/waybar-module.jsonc"
  install -Dm644 "scripts/config/waybar-style.css" "${pkgdir}/usr/share/${pkgname}/waybar-style.css"
}
