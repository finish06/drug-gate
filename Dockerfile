FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG VERSION=dev
ARG GIT_COMMIT=unknown
ARG GIT_BRANCH=unknown
RUN CGO_ENABLED=0 go build -ldflags="-s -w \
  -X github.com/finish06/drug-gate/internal/version.Version=${VERSION} \
  -X github.com/finish06/drug-gate/internal/version.GitCommit=${GIT_COMMIT} \
  -X github.com/finish06/drug-gate/internal/version.GitBranch=${GIT_BRANCH}" \
  -o /server ./cmd/server

FROM alpine:3.21

RUN apk --no-cache add ca-certificates
COPY --from=builder /server /server

EXPOSE 8081

ENTRYPOINT ["/server"]
