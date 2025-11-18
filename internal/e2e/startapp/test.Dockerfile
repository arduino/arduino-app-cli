FROM debian:trixie

RUN apt update && \
    apt install -y systemd systemd-sysv dbus \
    sudo docker.io ca-certificates curl gnupg \
    dpkg-dev apt-utils adduser gzip && \
    rm -rf /var/lib/apt/lists/*

ARG ARCH=amd64

COPY build/arduino-app-cli*_${ARCH}.deb /tmp/app.deb
COPY build/arduino-router*_${ARCH}.deb /tmp/router.deb

RUN apt update && apt install -y /tmp/router.deb /tmp/app.deb \
    && rm /tmp/app.deb /tmp/router.deb

RUN sed -i 's/--port 8800/--port 8800 --address 0.0.0.0/' /etc/systemd/system/arduino-app-cli.service

RUN usermod -s /bin/bash arduino || true
RUN mkdir -p /home/arduino && chown -R arduino:arduino /home/arduino
RUN usermod -aG docker arduino

EXPOSE 8800

CMD ["/sbin/init"]