ARG BASE_IMAGE=debian:trixie
FROM ${BASE_IMAGE}

RUN apt update && \
    apt install -y systemd systemd-sysv dbus initramfs-tools\
    sudo docker.io ca-certificates curl gnupg \
    dpkg-dev apt-utils adduser gzip && \
    rm -rf /var/lib/apt/lists/*

ARG ARCH=amd64

COPY build/stable/arduino-app-cli*_${ARCH}.deb /tmp/stable.deb
COPY build/arduino-app-cli*_${ARCH}.deb /tmp/unstable.deb
COPY build/stable/arduino-router*_${ARCH}.deb /tmp/router.deb

RUN apt update && apt install -y /tmp/stable.deb /tmp/router.deb \
    && rm /tmp/stable.deb /tmp/router.deb \
    && mkdir -p /var/www/html/myrepo/dists/local/main/binary-${ARCH} \
    && mv /tmp/unstable.deb /var/www/html/myrepo/dists/local/main/binary-${ARCH}/

WORKDIR /var/www/html/myrepo
RUN dpkg-scanpackages dists/local/main/binary-${ARCH} /dev/null | gzip -9c > dists/local/main/binary-${ARCH}/Packages.gz
WORKDIR /

# Debug level so the daemon's own warnings reach the journal. The drop-in is not
# owned by the package, so it survives the upgrade under test.
RUN mkdir -p /etc/systemd/system/arduino-app-cli.service.d && \
    printf '[Service]\nExecStart=\nExecStart=/usr/bin/arduino-app-cli daemon --port 8800 --log-level debug\n' \
    > /etc/systemd/system/arduino-app-cli.service.d/log-level.conf

# `system init` pulls the docker images and the arduino libraries of whichever
# version drives the upgrade, and the test only checks the version transition.
# Update the PATH so that the shim is found first.
ENV PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
RUN { echo '#!/bin/sh'; \
      echo 'if [ "$1" = system ] && [ "$2" = init ]; then exit 0; fi'; \
      echo 'exec /usr/bin/arduino-app-cli "$@"'; \
    } > /usr/local/bin/arduino-app-cli \
    && chmod +x /usr/local/bin/arduino-app-cli \
    && printf '[Service]\nEnvironment=PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\n' \
       > /etc/systemd/system/arduino-app-cli.service.d/skip-system-init.conf

RUN echo "deb [trusted=yes arch=${ARCH}] file:/var/www/html/myrepo local main" \
    > /etc/apt/sources.list.d/my-mock-repo.list

# Limit the tests to UNO Q (this reduces the number of pulled containers).
RUN echo '{"board_name": "unoq"}' > /var/lib/arduino-app-cli/platform.json

EXPOSE 8800
# CMD: systemd must be PID 1
CMD ["/sbin/init"]
