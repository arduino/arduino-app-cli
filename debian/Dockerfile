FROM golang:1.25.0 AS go

ARG BINARY_NAME
RUN test -n "${BINARY_NAME}" || (echo "Error: BINARY_NAME is not set" && exit 1)
ARG VERSION
RUN test -n "${VERSION}" || (echo "Error: VERSION is not set" && exit 1)
ARG ARCH
RUN test -n "${ARCH}" || (echo "Error: ARCH is not set" && exit 1)

# Download dependencies first (this layer will be cached by docker for quicker image builds).
# This will be recomputed only when these two files change, instead of every build.
COPY go.mod go.sum* /app/
WORKDIR /app

RUN --mount=type=cache,target=/go/pkg/mod,sharing=locked \
  go mod download

ADD cmd ./cmd
ADD internal ./internal
ADD pkg ./pkg

RUN --mount=type=cache,target=/go/pkg/mod,sharing=locked \
  --mount=type=cache,target=/root/.cache/go-build,sharing=locked \
  export VERSION=$(echo "${VERSION}" | sed -e "s/^v\(.*\)/\1/") \
  && GOOS=linux GOARCH=${ARCH} go build -ldflags "-X 'main.Version=${VERSION}'" -o ${BINARY_NAME} ./cmd/${BINARY_NAME}

FROM debian:bookworm AS debian

ARG VERSION
RUN test -n "${VERSION}" || (echo "Error: VERSION is not set" && exit 1)
ARG REVISION="1"
ARG ARCH
RUN test -n "${ARCH}" || (echo "Error: ARCH is not set" && exit 1)
ARG DEB_NAME
RUN test -n "${DEB_NAME}" || (echo "Error: DEB_NAME is not set" && exit 1)
ARG BINARY_NAME
RUN test -n "${BINARY_NAME}" || (echo "Error: BINARY_NAME is not set" && exit 1)

RUN apt-get update && apt-get install -y sed

COPY ./debian/${DEB_NAME} /${DEB_NAME}/
COPY --from=go /app/${BINARY_NAME} /${DEB_NAME}/usr/bin/${BINARY_NAME}

# Go application are tagged with `v` prefix, this remove the first v if present
RUN export VERSION=$(echo "${VERSION}" | sed -e "s/^v\(.*\)/\1/") && \
  sed -i "s/\$ARCH/${ARCH}/" /${DEB_NAME}/DEBIAN/control && \
  sed -i "s/\$VERSION/${VERSION}/" /${DEB_NAME}/DEBIAN/control && \
  dpkg-deb --build --root-owner-group /${DEB_NAME} &&\
  mv /${DEB_NAME}.deb "/${DEB_NAME}_${VERSION}-${REVISION}_${ARCH}.deb"

FROM scratch

COPY --from=debian /*.deb /
