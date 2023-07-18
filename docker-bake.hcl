variable "GO_LDFLAGS" {
  default = "-w -s"
}

group "default" {
  targets = ["build"]
}

target "build" {
  dockerfile = "Dockerfile"
  args = {
    GO_LDFLAGS = "${GO_LDFLAGS}"
  }
}

target "cross" {
  inherits = ["build"]
  platforms = ["linux/amd64", "linux/386", "linux/arm64", "linux/arm", "linux/ppc64le", "linux/s390x", "darwin/amd64", "darwin/arm64", "windows/amd64", "windows/arm64", "freebsd/amd64", "freebsd/arm64"]
  tags = ["ghcr.io/open-sauced/pizza"]
}