FROM golang:1.24.2 AS go

ARG BINARY_NAME="arduino-app-cli"

COPY . /app
WORKDIR /app
RUN GOOS=linux go build -o ${BINARY_NAME} ./cmd/arduino-app-cli/

FROM debian:bookworm AS debian

ARG VERSION="1.0.0"
ARG REVISION="1"

RUN apt-get update && apt-get install -y sed

COPY ./debian/orchestrator /orchestrator/
COPY --from=go /app/${BINARY_NAME} /orchestrator/usr/bin/${BINARY_NAME}

RUN sed -i "s/ARCH/$(dpkg --print-architecture)/" /orchestrator/DEBIAN/control && \
    dpkg-deb --build --root-owner-group /orchestrator &&\
    mv /orchestrator.deb "/orchestrator_${VERSION}-${REVISION}_$(dpkg --print-architecture).deb"

FROM scratch

COPY --from=debian /*.deb /
