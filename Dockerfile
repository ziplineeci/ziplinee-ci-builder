FROM docker:20.10.22-dind

ENV ZIPLINEE_CI_SERVER="ziplinee" \
    ZIPLINEE_WORKDIR="/ziplinee-work" \
    ZIPLINEE_LOG_FORMAT="v3" \
    ZIPLINEE_GIT_NAME="ziplinee-ci-builder"

LABEL maintainer="ziplinee.io" \
      description="The ${ZIPLINEE_GIT_NAME} is the component that runs builds as defined in the .ziplinee.yaml manifest"

RUN addgroup docker \
    && mkdir -p /ziplinee-entrypoints \
    && apk update \
    && apk add --no-cache --upgrade \
        openssl \
        apk-tools \
    && rm -rf /var/cache/apk/* \
    && docker version || true

# copy builder & startup script
COPY publish/ziplinee-ci-builder /
COPY templates /entrypoint-templates
COPY daemon.json /


WORKDIR /ziplinee-work

VOLUME /tmp
VOLUME /ziplinee-work

ENTRYPOINT ["/ziplinee-ci-builder"]
