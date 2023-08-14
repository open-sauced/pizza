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
  platforms = ["linux/amd64", "linux/arm64"]
  tags = ["ghcr.io/open-sauced/pizza"]
}
