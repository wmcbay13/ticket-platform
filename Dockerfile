FROM golang:1.27.1-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ cmd/
COPY internal/ internal/
ARG VERSION=development
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X main.version=${VERSION}" -o /ticket ./cmd/ticket

FROM gcr.io/distroless/static-debian12:nonroot
LABEL org.opencontainers.image.source="https://github.com/wmcbay13/ticket-platform"
COPY --from=build /ticket /ticket
EXPOSE 8080
USER 65532:65532
ENTRYPOINT ["/ticket"]
