pkgname=wizado
pkgver=0.1.0
pkgrel=1
pkgdesc="Omarchy-only: Hyprland hotkey + gamescope Steam couch mode helper (Arch Linux)"
arch=('any')
url="https://github.com/REPLACE_ME/wizado"
license=('MIT')
depends=('bash' 'curl' 'python')
optdepends=(
  'steam: Steam client'
  'gamescope: gamescope micro-compositor'
  'hyprland: Hyprland compositor (for keybind integration)'
  'jq: used by some Omarchy default binds (optional)'
)
install="${pkgname}.install"

source=("${pkgname}-${pkgver}.tar.gz::${url}/archive/refs/tags/v${pkgver}.tar.gz")
sha256sums=('SKIP')

package() {
  # The source tarball extracts to "wizado-${pkgver}" directory
  cd "${pkgname}-${pkgver}"

  install -d "${pkgdir}/usr/share/${pkgname}"
  cp -a scripts "${pkgdir}/usr/share/${pkgname}/"

  install -Dm755 "bin/wizado" "${pkgdir}/usr/bin/wizado"
}


