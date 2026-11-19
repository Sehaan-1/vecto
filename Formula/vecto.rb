class Vecto < Formula
  desc "Language-agnostic DAG task runner and content-addressed build cache"
  homepage "https://github.com/Sehaan-1/vecto"
  version "0.2.0"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/Sehaan-1/vecto/releases/download/v#{version}/vecto_#{version}_darwin_arm64.tar.gz"
      sha256 "c198816985fe9fa4240ed477e678c215d3d359b57674edb782e08c35e58f609f"
    else
      url "https://github.com/Sehaan-1/vecto/releases/download/v#{version}/vecto_#{version}_darwin_amd64.tar.gz"
      sha256 "e1b8b7a59498898761e6f5e497d3ae55d8bd4d3c86d20465fa1e8bebb652c181"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/Sehaan-1/vecto/releases/download/v#{version}/vecto_#{version}_linux_arm64.tar.gz"
      sha256 "3d3efb04a1c44847daa3a9e2a8b943c48f8fffe8df81a7bfcb7fa067fb924631"
    else
      url "https://github.com/Sehaan-1/vecto/releases/download/v#{version}/vecto_#{version}_linux_amd64.tar.gz"
      sha256 "5e78f61283bb0795ad592d0d414e758d86e1a9a47c1616d526ea71c70d8a77c7"
    end
  end

  def install
    bin.install "vecto"
  end

  test do
    assert_match "vecto version", shell_output("#{bin}/vecto --version")
  end
end
