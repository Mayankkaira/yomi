# Multi-stage build for the Go microservices. The build stage compiles the
# requested service binary; the runtime stage is a minimal distroless image.
# ARG SERVICE selects which cmd/<service> to build.

# ---- build stage ----
FROM golang:1.25-alpine AS build
ARG SERVICE
WORKDIR /src

# Cache dependencies first
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/yomi-${SERVICE} ./cmd/${SERVICE}

# ---- runtime stage ----
FROM gcr.io/distroless/static-debian12:nonroot AS runtime
ARG SERVICE
COPY --from=build /out/yomi-${SERVICE} /yomi-service
EXPOSE 8080
ENTRYPOINT ["/yomi-service"]