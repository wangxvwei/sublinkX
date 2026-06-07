FROM golang:1.22.2-alpine AS builder
WORKDIR /app
COPY . .
RUN go mod download
RUN go build -ldflags="-w -s" -o sublinkX

FROM alpine:latest
WORKDIR /app

ENV TZ=Asia/Shanghai

RUN apk add --no-cache ca-certificates tzdata \
    && mkdir -p /app/db /app/logs /app/template \
    && chmod 777 /app/db /app/logs /app/template

COPY --from=builder /app/sublinkX /app/sublinkX

EXPOSE 8000
CMD ["/app/sublinkX", "run", "--port", "8000"]
