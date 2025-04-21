ARG K6_VERSION="0.58.0"
FROM golang:1.24-alpine3.20 AS builder
# match whichever tagged version is used by the K6_VERSION docker image
# see build layer at https://github.com/grafana/k6/blob/v${K6_VERSION}/Dockerfile

ENV WORKSPACE=/src/dartboard/
WORKDIR $WORKSPACE

COPY [".", "$WORKSPACE"]

RUN apk update && \
    apk add bash tar unzip wget curl make

RUN go mod download && \
    go mod tidy && \
    go mod verify

RUN cd $WORKSPACE && \
    make && \
    mv ./dartboard /usr/local/bin/dartboard

# Copy bash and its dependencies
RUN mkdir -p /bash-layer/bin /bash-layer/lib && \
    cp /bin/bash /bash-layer/bin/ && \
    ldd /bin/bash | awk '{ print $3 }' | grep -v '^(' | xargs -I '{}' cp -v '{}' /bash-layer/lib/

FROM grafana/k6:${K6_VERSION}
COPY --from=builder /usr/local/bin/dartboard /bin/dartboard
# Copy bash binary and its dependencies
COPY --from=builder /bash-layer/bin/bash /bin/bash
COPY --from=builder /bash-layer/lib /lib

CMD [ "dartboard" ]
