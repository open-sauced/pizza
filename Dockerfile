FROM cgr.dev/chainguard/go:1.20

WORKDIR /app

# copy over the go source files
COPY ./ ./

# build the dependecies
RUN go mod download

# build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -o pizza-oven
