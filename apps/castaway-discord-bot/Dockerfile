FROM golang:1.24-bookworm AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/castaway-discord-bot ./cmd/bot

FROM gcr.io/distroless/base-debian12
WORKDIR /app
COPY --from=build /out/castaway-discord-bot /app/castaway-discord-bot

ENV CASTAWAY_API_BASE_URL=http://castaway-web:8080
ENV BOT_STATE_PATH=/app/data/state.db
ENV LOG_LEVEL=INFO

ENTRYPOINT ["/app/castaway-discord-bot"]
