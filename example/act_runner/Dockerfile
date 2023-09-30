FROM golang:1.21-alpine as runner

WORKDIR /build

RUN apk add make git

RUN git clone https://gitea.com/gitea/act_runner.git . && git reset --hard d07fbfc8c3

COPY ./example/act_runner/docker-client-ping.patch .

RUN git apply docker-client-ping.patch

RUN make build

FROM golang:1.21-alpine as proxy

WORKDIR /build

COPY . .

RUN go build .

FROM alpine

RUN mkdir /app /data && chown 10000:10000 /app /data

USER 10000

WORKDIR /data

COPY --from=runner /build/act_runner /usr/bin/act_runner
COPY --from=proxy /build/autoscaler-proxy /usr/bin/autoscaler-proxy

COPY ./example/act_runner/config.yml /app/config.yml

ENTRYPOINT ["autoscaler-proxy", "/app/config.yml"]
