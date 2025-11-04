FROM debian:trixie

ENV container docker
STOPSIGNAL SIGRTMIN+3
VOLUME ["/sys/fs/cgroup"]

# Install systemd + dependencies
RUN apt update && apt install -y systemd systemd-sysv dbus \
    dpkg-dev apt-utils adduser gzip \
    && rm -rf /var/lib/apt/lists/*

# Copy your packages and setup repo (as before)
ARG OLD_PACKAGE_PATH=build/old_package
ARG NEW_PACKAGE_PATH=build
ARG APP_PACKAGE_NAME=arduino-app-cli
ARG ROUTER_PACKAGE_NAME=arduino-router
ARG ARCH=arm64

COPY ${OLD_PACKAGE_PATH}/${APP_PACKAGE_NAME}*.deb /tmp/old_app.deb
COPY ${NEW_PACKAGE_PATH}/${APP_PACKAGE_NAME}*.deb /tmp/new_app.deb
COPY ${NEW_PACKAGE_PATH}/${ROUTER_PACKAGE_NAME}*.deb /tmp/new_router.deb

RUN apt update && apt install -y /tmp/old_app.deb /tmp/new_router.deb \
    && rm /tmp/old_app.deb \
    && mkdir -p /var/www/html/myrepo/dists/trixie/main/binary-${ARCH} \
    && mv /tmp/new_app.deb /var/www/html/myrepo/dists/trixie/main/binary-${ARCH}/ \
    && mv /tmp/new_router.deb /var/www/html/myrepo/dists/trixie/main/binary-${ARCH}/

WORKDIR /var/www/html/myrepo
RUN dpkg-scanpackages dists/trixie/main/binary-${ARCH} /dev/null | gzip -9c > dists/trixie/main/binary-${ARCH}/Packages.gz
WORKDIR /

RUN usermod -s /bin/bash arduino || true
RUN mkdir -p /home/arduino && chown -R arduino:arduino /home/arduino


RUN echo "deb [trusted=yes arch=${ARCH}] file:/var/www/html/myrepo trixie main" \
    > /etc/apt/sources.list.d/my-mock-repo.list


VOLUME [ "/sys/fs/cgroup" ]



# CMD: systemd must be PID 1
CMD ["/sbin/init"]
