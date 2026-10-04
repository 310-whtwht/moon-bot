# Build context: repository root (docker compose sets it), because the API
# imports packages/core through a replace directive.
FROM golang:1.24-alpine

RUN apk add --no-cache git ca-certificates curl

WORKDIR /src
COPY packages/core/go.mod packages/core/go.sum packages/core/
COPY apps/api/go.mod apps/api/go.sum apps/api/
WORKDIR /src/apps/api
RUN go mod download

WORKDIR /src
COPY packages/core packages/core
COPY apps/api apps/api
WORKDIR /src/apps/api
RUN go build -o /app/main .

WORKDIR /app
EXPOSE 8081
CMD ["./main"]
