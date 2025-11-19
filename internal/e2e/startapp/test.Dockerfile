FROM debian:trixie

# Install packages first
RUN apt update && \
    apt install -y systemd systemd-sysv dbus \
                   sudo docker.io docker-compose \
                   ca-certificates curl gnupg \
                   dpkg-dev apt-utils adduser gzip && \
    rm -rf /var/lib/apt/lists/*

ARG ARCH=amd64

COPY build/arduino-app-cli*_${ARCH}.deb /tmp/app.deb
COPY build/arduino-router*_${ARCH}.deb /tmp/router.deb

RUN apt update && apt install -y /tmp/router.deb /tmp/app.deb \
    && rm /tmp/app.deb /tmp/router.deb

RUN usermod -s /bin/bash arduino || true
RUN mkdir -p /home/arduino && chown -R arduino:arduino /home/arduino
RUN usermod -aG docker arduino

# Copy + enable mock devices script
COPY create-mock-devices.sh /usr/local/bin/create-mock-devices.sh
RUN chmod +x /usr/local/bin/create-mock-devices.sh

EXPOSE 8800

# ENTRYPOINT must remain last
ENTRYPOINT ["/usr/local/bin/create-mock-devices.sh"]
CMD ["/sbin/init"]
