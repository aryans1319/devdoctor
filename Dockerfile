FROM golang:1.23-alpine AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o devdoctor .

FROM alpine:3.19
WORKDIR /app
COPY --from=build /app/devdoctor .
EXPOSE 8080
CMD ["./devdoctor", "serve"]
