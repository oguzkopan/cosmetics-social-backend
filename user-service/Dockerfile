FROM golang:1.22-bookworm AS build
WORKDIR /src
COPY . .
RUN go build -o /usr/local/bin/server .

FROM gcr.io/distroless/base-debian12
COPY --from=build /usr/local/bin/server /server
EXPOSE 8080
ENTRYPOINT ["/server"]
