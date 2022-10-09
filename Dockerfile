###########
# builder #
###########

FROM golang:1.19-buster AS builder
RUN apt-get update \
    && apt-get install -y --no-install-recommends \
    upx-ucl

WORKDIR /build
COPY . .

RUN GO111MODULE=on CGO_ENABLED=0 go build \
      -ldflags='-w -s -extldflags "-static"' \
      -o ./bin/my_monkey \
 && upx-ucl --best --ultra-brute ./bin/my_monkey

###########
# release #
###########

FROM golang:1.19-buster AS release
RUN apt-get update \
    && apt-get install -y --no-install-recommends \
    git

COPY --from=builder /build/bin/my_monkey /bin/
WORKDIR /workdir
ENTRYPOINT ["/bin/my_monkey"]
