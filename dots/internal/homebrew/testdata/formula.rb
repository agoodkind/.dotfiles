class {{.ClassName}} < Formula
  desc "Installs a local maintenance test artifact"
  homepage "https://example.com"
  url "{{.SourceURL}}"
  version "{{.Version}}"
  sha256 "{{.SHA256}}"
  license "MIT"

  def install
{{if .Fail}}    odie "maintenance fixture upgrade failed"
{{else}}    pkgshare.install "payload.txt"
{{end}}  end
end
