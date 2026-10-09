FROM golang:1.27 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN make build

FROM gcr.io/distroless/static-debian13:nonroot
COPY --from=build /src/bin/collector /src/bin/api /
USER nonroot
ENTRYPOINT ["/api"]
