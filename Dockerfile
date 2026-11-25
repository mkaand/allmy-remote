FROM golang:1.26.11-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download && go mod verify
COPY cmd ./cmd
COPY internal ./internal
RUN go mod tidy && CGO_ENABLED=0 go build -mod=readonly -trimpath -ldflags="-s -w" -o /out/allmy-remote ./cmd/server

RUN mkdir -m 0700 /out/data

FROM scratch
COPY --from=build /out/allmy-remote /allmy-remote
COPY --from=build --chown=65532:65532 /out/data /data
USER 65532:65532
ENV ALLMY_LISTEN=0.0.0.0:8080 ALLMY_ALLOW_NON_LOOPBACK=true ALLMY_DATA_DIR=/data
EXPOSE 8080
ENTRYPOINT ["/allmy-remote"]
