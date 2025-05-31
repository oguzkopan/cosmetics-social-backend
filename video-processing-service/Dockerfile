# Build ffmpeg-enabled image
FROM golang:1.22-bookworm AS build
RUN apt-get update && apt-get install -y ffmpeg
WORKDIR /src
COPY . .
RUN go build -o /usr/local/bin/server .

FROM gcr.io/distroless/base-debian12
COPY --from=build /usr/local/bin/server /server
COPY --from=build /usr/bin/ffmpeg /usr/bin/ffmpeg
EXPOSE 8080
ENTRYPOINT ["/server"]
