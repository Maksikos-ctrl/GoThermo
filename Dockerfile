# Builds the headless GoThermo server (no window, no GUI libraries).
FROM golang:1.23 AS build
ENV GOTOOLCHAIN=auto
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -tags server -o /gothermo-server .

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates \
    && rm -rf /var/lib/apt/lists/*
COPY --from=build /gothermo-server /usr/local/bin/gothermo-server
# Uploaded files are saved relative to the working directory.
WORKDIR /data
ENV GOTHERMO_ADDR=0.0.0.0:8081
EXPOSE 8081
CMD ["gothermo-server"]
