# syntax=docker/dockerfile:1
FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/confighub ./cmd/confighub

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/confighub /usr/local/bin/confighub
WORKDIR /var/lib/confighub
USER nonroot
EXPOSE 8787
ENTRYPOINT ["/usr/local/bin/confighub"]
CMD ["serve", "--bind", "0.0.0.0:8787", "--root", "/var/lib/confighub"]
