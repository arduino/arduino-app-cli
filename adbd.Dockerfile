FROM debian:trixie

RUN apt-get update \
    && apt-get install -y --no-install-recommends adbd file ssh sudo \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/*

RUN useradd -m --create-home --shell /bin/bash --user-group --groups sudo arduino && \
    echo "arduino:arduino" | chpasswd && \
    mkdir /home/arduino/ArduinoApps && \
    chown -R arduino:arduino /home/arduino/ArduinoApps

WORKDIR /home/arduino
EXPOSE 22

CMD ["/bin/sh", "-c", "/usr/sbin/sshd -D & su arduino -c adbd"]
