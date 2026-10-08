# Build from the repo root: docker build -f apps/probst/Dockerfile .  (bakes in seasons/ for scheduled runs)
FROM golang:1.24-bookworm AS build
WORKDIR /src
COPY apps/probst/go.mod apps/probst/go.sum ./
RUN go mod download
COPY apps/probst/ .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/probst .

FROM gcr.io/distroless/base-debian12
WORKDIR /app
COPY --from=build /out/probst /app/probst
COPY seasons/ /app/seasons/
ENV PROBST_API_URL=http://castaway-web:8080
ENTRYPOINT ["/app/probst"]
