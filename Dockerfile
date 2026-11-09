ARG GO_VERSION=1.26.1

# 阶段1：下载依赖
FROM golang:${GO_VERSION}-alpine AS deps
WORKDIR /src
ENV GOPROXY=https://goproxy.cn,direct  
ENV GOSUMDB=sum.golang.google.cn 
COPY go.mod go.sum ./
RUN go mod download

# 阶段2：源码
FROM deps AS build
COPY . ./
ENV CGO_ENABLED=0

# 阶段3：两个编译阶段
FROM build AS api-build
RUN go build -trimpath -ldflags="-s -w" -o /out/api .

FROM build AS worker-build
RUN go build -trimpath -ldflags="-s -w" -o /out/worker ./cmd/worker

# 阶段4：运行底座
FROM alpine:3.21 AS base
RUN sed -i 's#dl-cdn.alpinelinux.org#mirrors.aliyun.com#g' /etc/apk/repositories && \
    apk add --no-cache ca-certificates tzdata && \
    adduser -D -H -s /sbin/nologin app
ENV TZ=Asia/Shanghai
RUN ln -snf /usr/share/zoneinfo/$TZ /etc/localtime && echo $TZ > /etc/timezone
WORKDIR /app
COPY --from=build /src/config.docker.yaml /app/config.yaml
COPY --from=build /src/frontend /app/frontend
RUN mkdir -p /app/uploads/avatars && chown -R app:app /app
USER app

FROM base AS api
COPY --from=api-build /out/api /app/api
EXPOSE 8080
ENTRYPOINT ["/app/api"]

FROM base AS worker
COPY --from=worker-build /out/worker /app/worker
ENTRYPOINT [ "/app/worker" ]