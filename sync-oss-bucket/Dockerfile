FROM golang:1.21.4 as builder
WORKDIR /build
RUN go env -w GOPROXY=https://goproxy.cn

COPY . .
ENV CGO_ENABLED=0
RUN go build

FROM sh-harbor.mthreads.com/cloud/distroless/static:noroot
WORKDIR /
COPY --from=builder /build/sync-bucket .