class Cliquest < Formula
  desc "Practical terminal troubleshooting training for engineers"
  homepage "https://github.com/tensho1026/CLI-Quest"
  url "https://github.com/tensho1026/CLI-Quest.git", tag: "v0.2.0"
  version "0.2.0"
  license "MIT"

  depends_on "go" => :build

  def install
    ldflags = "-s -w -X github.com/tensho1026/CLI-Quest/internal/cli.Version=v#{version}"
    system "go", "build", *std_go_args(ldflags: ldflags), "./cmd/cliquest"
  end

  test do
    assert_match "cliquest v#{version}", shell_output("#{bin}/cliquest version")
    assert_match "linux-permission", shell_output("CLIQUEST_HOME=#{testpath}/state #{bin}/cliquest list")
  end
end
