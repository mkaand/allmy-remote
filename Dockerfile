FROM node:24.22.3-bookworm-slim AS web
WORKDIR /src/web
COPY web/package.json ./
RUN npm install --ignore-scripts --no-audit --no-fund
COPY web/tsconfig.json ./
COPY web/src ./src
RUN npm run typecheck && npm run build

FROM golang:1.26.11-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download && go mod verify
COPY cmd ./cmd
COPY internal ./internal
COPY --from=web /src/internal/core/static/app.js ./internal/core/static/app.js
RUN CGO_ENABLED=0 go build -mod=readonly -trimpath -ldflags="-s -w" -o /out/allmy-remote ./cmd/server

RUN mkdir -m 0700 /out/data

FROM scratch
COPY --from=build /out/allmy-remote /allmy-remote
COPY --from=build --chown=65532:65532 /out/data /data
USER 65532:65532
ENV ALLMY_LISTEN=0.0.0.0:8080 ALLMY_ALLOW_NON_LOOPBACK=true ALLMY_DATA_DIR=/data
EXPOSE 8080
ENTRYPOINT ["/allmy-remote"]
