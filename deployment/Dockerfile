FROM golang:1.23 AS builder
WORKDIR /app
COPY ./go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o ./bin/main ./cmd/main.go

FROM scratch

ARG VERSION
ARG SHA
ENV VERSION=${VERSION}
ENV SHA=${SHA}

WORKDIR /root/
COPY --from=builder /app/bin/main ./bin/main
EXPOSE 1960
ENV PERSONAL_SITE_ADDR="0.0.0.0:1960"
CMD ["bin/main"]
HEALTHCHECK CMD ["bin/main","healthcheck"]
