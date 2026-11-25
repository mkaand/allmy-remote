FROM golang:1.26.11-bookworm AS build
WORKDIR /src
COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/allmy-remote ./cmd/server

FROM scratch
COPY --from=build /out/allmy-remote /allmy-remote
USER 65532:65532
ENV ALLMY_LISTEN=0.0.0.0:8080 ALLMY_ALLOW_NON_LOOPBACK=true
EXPOSE 8080
ENTRYPOINT ["/allmy-remote"]
