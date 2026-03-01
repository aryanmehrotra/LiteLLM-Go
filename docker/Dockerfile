FROM golang:1.25-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /app/llm-gateway .

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata
COPY --from=build /app/llm-gateway /llm-gateway
COPY --from=build /src/configs /configs
COPY --from=build /src/admin/static /admin/static
COPY --from=build /src/templates /templates

EXPOSE 9000 2121

CMD ["/llm-gateway"]
