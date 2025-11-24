FROM debian:trixie

RUN apt update && \
    apt install -y systemd systemd-sysv dbus \
    sudo docker.io ca-certificates curl gnupg \
    dpkg-dev apt-utils adduser gzip && \
    rm -rf /var/lib/apt/lists/*

ARG ARCH=amd64

COPY build/stable/arduino-app-cli*_${ARCH}.deb /tmp/stable.deb
COPY build/arduino-app-cli*_${ARCH}.deb /tmp/unstable.deb
COPY build/stable/arduino-router*_${ARCH}.deb /tmp/router.deb

RUN apt update && apt install -y /tmp/stable.deb /tmp/router.deb \
    && rm /tmp/stable.deb /tmp/router.deb \
    && mkdir -p /var/www/html/myrepo/dists/trixie/main/binary-${ARCH} \
    && mv /tmp/unstable.deb /var/www/html/myrepo/dists/trixie/main/binary-${ARCH}/

WORKDIR /var/www/html/myrepo
RUN dpkg-scanpackages dists/trixie/main/binary-${ARCH} /dev/null | gzip -9c > dists/trixie/main/binary-${ARCH}/Packages.gz
WORKDIR /

RUN usermod -s /bin/bash arduino || true
RUN mkdir -p /home/arduino && chown -R arduino:arduino /home/arduino
RUN usermod -aG docker arduino

RUN echo "deb [trusted=yes arch=${ARCH}] file:/var/www/html/myrepo trixie main" \
    > /etc/apt/sources.list.d/my-mock-repo.list

EXPOSE 8800
# CMD: systemd must be PID 1
CMD ["/sbin/init"]
