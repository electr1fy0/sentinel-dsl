FROM golang:1.22 AS build
WORKDIR /src
COPY go.mod ./
COPY . .
RUN CGO_ENABLED=0 go build -o /sentinel ./cmd/sentinel

FROM gcr.io/distroless/static-debian12
COPY --from=build /sentinel /sentinel
ENTRYPOINT ["/sentinel"]
