FROM golang:1.26.10 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /xochi ./cmd/guardian

FROM gcr.io/distroless/static
COPY --from=build /xochi /xochi
ENTRYPOINT ["/xochi"]
