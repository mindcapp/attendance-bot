FROM golang:1.26-alpine AS build
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /app/bin/attendance-bot .

FROM alpine:3.20
WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata

COPY --from=build /app/bin/attendance-bot /app/bin/attendance-bot
COPY --from=build /app/data /app/data

EXPOSE 9090

ENTRYPOINT ["/app/bin/attendance-bot"]
