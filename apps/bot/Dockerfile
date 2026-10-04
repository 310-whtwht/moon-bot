# Build context: repository root (docker compose sets it), because the bot
# imports packages/core through a replace directive.
FROM golang:1.24-alpine AS build

WORKDIR /src
COPY packages/core/go.mod packages/core/go.sum packages/core/
COPY apps/bot/go.mod apps/bot/go.sum apps/bot/
WORKDIR /src/apps/bot
RUN go mod download

WORKDIR /src
COPY packages/core packages/core
COPY apps/bot apps/bot
WORKDIR /src/apps/bot
RUN CGO_ENABLED=0 go build -o /out/bot .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=build /out/bot /usr/local/bin/bot
ENTRYPOINT ["bot"]
