# Builds the browser version of the interface.
FROM node:20-alpine AS web
WORKDIR /src/frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ ./
ENV VITE_WEB=1
RUN npm run build

# Builds the headless GoThermo server (no window, no GUI libraries).
FROM golang:1.23 AS build
ENV GOTOOLCHAIN=auto
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -tags server -o /gothermo-server . \
    && go build -o /gothermo-admin ./cmd/gothermo-admin

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates \
    && rm -rf /var/lib/apt/lists/*
COPY --from=build /gothermo-server /usr/local/bin/gothermo-server
COPY --from=build /gothermo-admin /usr/local/bin/gothermo-admin
COPY --from=web /src/frontend/dist /srv/web/app
# Uploaded files are saved relative to the working directory.
WORKDIR /data
ENV GOTHERMO_ADDR=0.0.0.0:8081
ENV GOTHERMO_WEB_DIR=/srv/web
EXPOSE 8081
CMD ["gothermo-server"]
