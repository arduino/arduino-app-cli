FROM golang:1.24.2 AS go

ARG BINARY_NAME
RUN test -n "${BINARY_NAME}" || (echo "Error: BINARY_NAME is not set" && exit 1)

COPY . /app
WORKDIR /app
RUN GOOS=linux go build -o ${BINARY_NAME} ./cmd/${BINARY_NAME}

FROM debian:bookworm AS debian

ARG VERSION="1.0.0"
ARG REVISION="1"
ARG DEB_NAME
RUN test -n "${DEB_NAME}" || (echo "Error: DEB_NAME is not set" && exit 1)
ARG BINARY_NAME
RUN test -n "${BINARY_NAME}" || (echo "Error: BINARY_NAME is not set" && exit 1)

RUN apt-get update && apt-get install -y sed

COPY ./debian/${DEB_NAME} /${DEB_NAME}/
COPY --from=go /app/${BINARY_NAME} /${DEB_NAME}/usr/bin/${BINARY_NAME}

RUN sed -i "s/ARCH/$(dpkg --print-architecture)/" /${DEB_NAME}/DEBIAN/control && \
    dpkg-deb --build --root-owner-group /${DEB_NAME} &&\
    mv /${DEB_NAME}.deb "/${DEB_NAME}_${VERSION}-${REVISION}_$(dpkg --print-architecture).deb"

FROM scratch

COPY --from=debian /*.deb /
