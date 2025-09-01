FROM golang:1.24-alpine

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY cmd ./cmd
COPY internal/ ./internal/

# Build with -ldflags="-s -w" to strip debugging symbols and DWARF tables
RUN go build -ldflags="-s -w" -o zonocaller ./cmd/zonocaller

# Make dir for logs
RUN mkdir -p /app/data

# Set env variables
ENV API_URL=https://api.ipify.org?format=json
ENV OUTPUT_FILE=/app/data/ip_log.txt
ENV MAX_RETRIES=3
ENV TIMEZONE=Europe/London
ENV SCHEDULE_TIME=23:59

############################### CHANGE ME  ############################### 
ENV ZONOMI_HOSTS=example.com,another.host
ENV ZONOMI_API_KEY=
ENV ZONOMI_API_ENCRYPTED=false
ENV ZONOMI_ENCRYPT_KEY=

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --quiet --tries=1 --spider http://localhost:8000/health || exit 1

# Run
CMD ["./zonocaller"]
