# ─── build stage ────────────────────────────────────────────────────────
FROM golang:1.24-bookworm AS build

RUN apt-get update && \
    DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends ffmpeg && \
    rm -rf /var/lib/apt/lists/*

WORKDIR /src

# copy go mod files and download deps (classic Docker syntax)
COPY go.mod go.sum ./
RUN go mod download

# copy the rest
COPY . .

# produce static binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -o /out/server .

# ─── runtime stage ──────────────────────────────────────────────────────
FROM gcr.io/distroless/base-debian12
COPY --from=build /out/server      /server
COPY --from=build /usr/bin/ffmpeg  /usr/bin/ffmpeg
EXPOSE 8080
ENTRYPOINT ["/server"]
