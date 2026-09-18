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

# Fixture packages for the three plans the apt resolver can report: a package it
# holds back, an upgrade that needs a new package, and a pinned downgrade.
ARG EXTRA_PACKAGES=0
RUN if [ "${EXTRA_PACKAGES}" = 1 ]; then set -eu; \
      repo=/var/www/html/myrepo/dists/local/main/binary-${ARCH}; \
      mkdeb() { \
        rm -rf /tmp/fixture && mkdir -p /tmp/fixture/DEBIAN; \
        { printf 'Package: %s\nVersion: %s\nArchitecture: all\n' "$1" "$2"; \
          printf 'Maintainer: Arduino <test@arduino.cc>\n'; \
          [ -z "$3" ] || printf '%s\n' "$3"; \
          printf 'Description: apt resolver test fixture\n'; \
        } > /tmp/fixture/DEBIAN/control; \
        dpkg-deb -b /tmp/fixture "$repo/$1_$2_all.deb"; \
      }; \
      mkdeb arduino-heldback-test 1.0 ''; \
      mkdeb arduino-heldback-test 2.0 'Depends: arduino-absent-test'; \
      mkdeb arduino-newdep-test 1.0 ''; \
      mkdeb arduino-newdep-test 2.0 'Depends: arduino-newdep-lib-test'; \
      mkdeb arduino-newdep-lib-test 1.0 ''; \
      mkdeb arduino-rec-test 1.0 ''; \
      mkdeb arduino-rec-test 2.0 'Recommends: arduino-rec-lib-test'; \
      mkdeb arduino-rec-lib-test 1.0 ''; \
      mkdeb arduino-down-test 1.0 ''; \
      mkdeb arduino-down-test 2.0 ''; \
      dpkg -i "$repo"/arduino-heldback-test_1.0_all.deb \
              "$repo"/arduino-newdep-test_1.0_all.deb \
              "$repo"/arduino-rec-test_1.0_all.deb \
              "$repo"/arduino-down-test_2.0_all.deb; \
      printf 'Package: arduino-down-test\nPin: version 1.0\nPin-Priority: 1001\n' \
        > /etc/apt/preferences.d/arduino-down-test; \
    fi

WORKDIR /var/www/html/myrepo
# -m publishes every version, so the repo can hold both ends of a downgrade.
RUN dpkg-scanpackages -m dists/local/main/binary-${ARCH} /dev/null | gzip -9c > dists/local/main/binary-${ARCH}/Packages.gz
WORKDIR /

# Debug level so the daemon's own warnings reach the journal. The drop-in is not
# owned by the package, so it survives the upgrade under test.
RUN mkdir -p /etc/systemd/system/arduino-app-cli.service.d && \
    printf '[Service]\nExecStart=\nExecStart=/usr/bin/arduino-app-cli daemon --port 8800 --log-level debug\n' \
    > /etc/systemd/system/arduino-app-cli.service.d/log-level.conf

# `system init` pulls the docker images and the arduino libraries of whichever
# version drives the upgrade, and the test only checks the version transition.
# One CI leg builds with SKIP_SYSTEM_INIT=0 to run the real command.
ARG SKIP_SYSTEM_INIT=1
# Update the PATH so that the shim is found first.
ENV PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
RUN if [ "${SKIP_SYSTEM_INIT}" = 1 ]; then \
      { echo '#!/bin/sh'; \
        echo 'if [ "$1" = system ] && [ "$2" = init ]; then exit 0; fi'; \
        echo 'exec /usr/bin/arduino-app-cli "$@"'; \
      } > /usr/local/bin/arduino-app-cli \
      && chmod +x /usr/local/bin/arduino-app-cli \
      && printf '[Service]\nEnvironment=PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\n' \
         > /etc/systemd/system/arduino-app-cli.service.d/skip-system-init.conf; \
    fi

RUN echo "deb [trusted=yes arch=${ARCH}] file:/var/www/html/myrepo local main" \
    > /etc/apt/sources.list.d/my-mock-repo.list

EXPOSE 8800
# CMD: systemd must be PID 1
CMD ["/sbin/init"]
