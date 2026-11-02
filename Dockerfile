FROM node:22-alpine AS web-builder

WORKDIR /web

COPY web/package*.json ./
RUN npm ci

COPY web ./
RUN npm run build

FROM golang:1.27-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
COPY --from=web-builder /web/dist/ ./internal/api/static/
RUN CGO_ENABLED=0 go build -o /out/jp .

FROM alpine:3.22

RUN apk add --no-cache tzdata
RUN addgroup -S app && adduser -S app -G app

WORKDIR /app
COPY --from=builder /out/jp /app/jp

RUN mkdir -p /app/content && chown -R app:app /app
USER app

EXPOSE 8080
ENTRYPOINT ["/app/jp"]
