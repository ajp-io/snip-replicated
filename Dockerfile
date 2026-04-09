# Build stage
FROM --platform=linux/amd64 golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /snip ./cmd/snip

# Download support-bundle binary for in-app bundle generation (task 3.7)
ARG TROUBLESHOOT_VERSION=0.125.1
RUN wget -q -O /tmp/sb.tar.gz \
      "https://github.com/replicatedhq/troubleshoot/releases/download/v${TROUBLESHOOT_VERSION}/support-bundle_linux_amd64.tar.gz" \
  && tar -xzf /tmp/sb.tar.gz -C /usr/local/bin support-bundle \
  && chmod +x /usr/local/bin/support-bundle \
  && rm /tmp/sb.tar.gz

# Runtime stage
FROM --platform=linux/amd64 alpine:3.20
RUN apk --no-cache add ca-certificates tzdata
WORKDIR /app
COPY --from=builder /snip /app/snip
COPY --from=builder /usr/local/bin/support-bundle /usr/local/bin/support-bundle
EXPOSE 8080
ENTRYPOINT ["/app/snip"]
