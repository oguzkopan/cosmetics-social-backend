# ─── build stage ────────────────────────────────────────────────────────
FROM golang:1.24-bookworm AS build             

WORKDIR /src

# Copy go.mod/go.sum first so the download layer is cached
COPY go.mod go.sum ./
RUN go mod download

# Now copy the rest of the source
COPY . .

# Build a static Linux binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -v -o /out/server .

# ─── runtime stage ──────────────────────────────────────────────────────
FROM gcr.io/distroless/base-debian12

COPY --from=build /out/server /server

ENTRYPOINT ["/server"]
