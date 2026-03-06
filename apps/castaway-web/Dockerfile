FROM golang:1.24-bookworm AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/castaway-web ./cmd/server

FROM gcr.io/distroless/base-debian12
WORKDIR /app
COPY --from=build /out/castaway-web /app/castaway-web
COPY db/migrations /app/db/migrations

ENV PORT=8080
ENV AUTO_MIGRATE=true
ENV MIGRATIONS_DIR=/app/db/migrations

EXPOSE 8080
ENTRYPOINT ["/app/castaway-web"]
