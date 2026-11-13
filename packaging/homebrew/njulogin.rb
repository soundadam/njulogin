# frozen_string_literal: true

# scripts/render-cask.sh fills in the version and the SHA-256 of each release
# archive; the release workflow pushes the result to soundadam/homebrew-tap as
# Casks/njulogin.rb.
cask "njulogin" do
  arch arm: "aarch64", intel: "x86_64"
  os macos: "macos", linux: "linux"

  version "@VERSION@"
  sha256 arm:          "@MACOS_AARCH64_SHA256@",
         intel:        "@MACOS_X86_64_SHA256@",
         arm64_linux:  "@LINUX_AARCH64_SHA256@",
         x86_64_linux: "@LINUX_X86_64_SHA256@"

  on_macos do
    # Ad-hoc signed and not notarized: drop the quarantine attribute so
    # Gatekeeper treats the Cask install like a self-updated binary.
    postflight_steps do
      run "/usr/bin/xattr", args: ["-dr", "com.apple.quarantine", "{{staged_path}}"]
    end
  end

  url "https://github.com/soundadam/njulogin/releases/download/v#{version}/njulogin-#{version}-#{os}-#{arch}.tar.gz"
  name "NJUlogin"
  desc "Authenticate local or remote network paths with the NJU portal"
  homepage "https://github.com/soundadam/njulogin"

  livecheck do
    url :url
    strategy :github_latest
  end

  binary "njulogin-#{version}-#{os}-#{arch}/njulogin"
end
