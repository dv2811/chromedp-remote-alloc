FROM golang:1.25.2-trixie AS builder
WORKDIR /cmd
COPY . .
# Build the Go app
RUN CGO_ENABLED=0 GOOS=linux go test -c ./cmd -o ./browser.test
# Use the built Go app in a minimal image
FROM chromedp/headless-shell:latest
WORKDIR /cmd
RUN apt-get update && apt-get install -y ca-certificates \
	&& apt-get clean && rm -rf /var/lib/apt/lists/*

# Create writable user data directory
RUN mkdir -p /tmp/chromium-data && chmod 777 /tmp/chromium-data

# Expose debug port
EXPOSE 9222 8080

COPY --from=builder /cmd/browser.test /cmd/browser.test
ENV PATH=/headless-shell:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:/cmd
RUN chmod +x /cmd/browser.test
ENTRYPOINT ["/cmd/browser.test", "--headless-mode=true"]