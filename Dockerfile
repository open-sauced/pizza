FROM --platform=${BUILDPLATFORM:-linux/amd64} cgr.dev/chainguard/go:1.20 AS builder
ARG TARGETPLATFORM
ARG BUILDPLATFORM
ARG TARGETOS
ARG TARGETARCH

WORKDIR /app
COPY . .
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -o pizza-oven .

FROM --platform=${TARGETPLATFORM:-linux/amd64} cgr.dev/chainguard/glibc-dynamic
COPY --from=builder /app/pizza-oven /usr/bin/
CMD ["/usr/bin/pizza-oven"]
