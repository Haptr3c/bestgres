ARG BUILDPLATFORM
# Build the manager binary
FROM --platform=${BUILDPLATFORM} golang:1.22 AS builder

WORKDIR /app
# Copy the go.mod and go.sum files
COPY go.mod go.sum ./
# Download the go module dependencies
RUN go mod download

# Copy the entire project directory
COPY . .

ARG TARGETARCH
ARG TARGETOS
# Build
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -a -o operator cmd/operator/main.go

# Use a minimal image
FROM scratch

# Copy the manager binary
COPY --from=builder /app/operator /operator

# Run the manager binary
ENTRYPOINT ["/operator"]