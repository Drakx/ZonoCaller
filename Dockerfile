FROM golang:1.23-alpine

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY cmd ./cmd
COPY internal/ ./internal/

# Build
RUN go build -s zonocaller ./cmd/zonocaller

# Make dir for logs
RUN mkdir -p /app/data

# Set env variables
ENV API_URL=https://api.ipify.org?format=json
ENV OUTPUT_FILE=/app/data/ip_log.txt
ENV MAX_RETRIES=3
ENV TIMEZONE=Europe/London
ENV SCHEDULE_TIME=23:59
ENV ZONOMI_HOST=example.com  # DID YOU CHANGE ME?
ENV ZONOMI_API_KEY=
ENV ZONOMI_API_ENCRYPTED=false
ENV ZONOMI_ENCRYPT_KEY=

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --quiet --tries=1 --spider http://localhost:8080/health || exit 1

# Run
CMD ["./zonocaller"]
