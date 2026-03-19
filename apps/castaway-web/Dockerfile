FROM golang:1.24-bookworm AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
ARG VERSION=dev
ARG GIT_COMMIT=unknown
ARG BUILD_DATE=unknown
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "-X github.com/bry-guy/srvivor/apps/castaway-web/internal/buildinfo.Version=${VERSION} -X github.com/bry-guy/srvivor/apps/castaway-web/internal/buildinfo.Commit=${GIT_COMMIT} -X github.com/bry-guy/srvivor/apps/castaway-web/internal/buildinfo.BuildDate=${BUILD_DATE}" -o /out/castaway-web ./cmd/server \
 && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "-X github.com/bry-guy/srvivor/apps/castaway-web/internal/buildinfo.Version=${VERSION} -X github.com/bry-guy/srvivor/apps/castaway-web/internal/buildinfo.Commit=${GIT_COMMIT} -X github.com/bry-guy/srvivor/apps/castaway-web/internal/buildinfo.BuildDate=${BUILD_DATE}" -o /out/castaway-web-migrate ./cmd/migrate

FROM gcr.io/distroless/base-debian12
WORKDIR /app
COPY --from=build /out/castaway-web /app/castaway-web
COPY --from=build /out/castaway-web-migrate /app/castaway-web-migrate
COPY db/migrations /app/db/migrations

ENV PORT=8080
ENV AUTO_MIGRATE=false
ENV MIGRATIONS_DIR=/app/db/migrations
ENV SERVICE_AUTH_ENABLED=false
ENV SERVICE_AUTH_PRINCIPAL=castaway-discord-bot

EXPOSE 8080
ENTRYPOINT ["/app/castaway-web"]
