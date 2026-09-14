# syntax=docker/dockerfile:1
FROM node:22-bookworm-slim AS frontend
WORKDIR /src/frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/index.html frontend/tsconfig*.json frontend/vite.config.ts ./
COPY frontend/src ./src
RUN npm run build

FROM --platform=$BUILDPLATFORM golang:1.27-bookworm AS backend
ARG TARGETOS
ARG TARGETARCH
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY backend ./backend
COPY cmd/yak8sui ./cmd/yak8sui
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} \
    go build -trimpath -ldflags="-s -w" -o /out/yak8sui ./cmd/yak8sui

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=backend /out/yak8sui /app/yak8sui
COPY --from=frontend /src/frontend/dist /app/frontend
USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/app/yak8sui"]
CMD ["-config", "/etc/yak8sui/config.yaml"]
