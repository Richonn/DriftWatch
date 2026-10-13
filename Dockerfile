FROM golang:1.27-alpine AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /driftwatch ./cmd/driftwatch

FROM gcr.io/distroless/static:nonroot

COPY --from=builder /driftwatch /driftwatch

ENTRYPOINT ["/driftwatch"]
