ARG BASE_IMAGE=debian:11-slim
ARG GO_IMAGE=golang:latest
FROM $GO_IMAGE as builder

# NOTE: This arg will be populated by docker buildx
# https://docs.docker.com/engine/reference/builder/#automatic-platform-args-in-the-global-scope
ARG TARGETARCH

RUN mkdir -p /go/src/github.com/daniel-widrick/mcPortKnock/
WORKDIR /go/src/github.com/daniel-widrick/mcPortKnock/
COPY . /go/src/github.com/daniel-widrick/mcPortKnock/

ARG GO111MODULE="on"
ENV GO111MODULE=${GO111MODULE}

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -buildmode=default -ldflags="-s -w"

FROM $BASE_IMAGE

ARG USER="nobody"
ARG GROUP="nobody"

COPY --from=builder --chown=${USER}:${GROUP} /go/src/github.com/daniel-widrick/mcPortKnock/mcPortKnock /mcPortKnock

USER ${USER}

ENTRYPOINT ["/mcPortKnock"]
