FROM debian:trixie

RUN apt-get update \
    && apt-get install -y --no-install-recommends adbd file ssh sudo netcat-traditional \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/*

RUN useradd -m --create-home --shell /bin/bash --user-group --groups sudo arduino && \
    echo "arduino:arduino" | chpasswd && \
    mkdir /home/arduino/ArduinoApps && \
    chown -R arduino:arduino /home/arduino/ArduinoApps

ADD scripts/pong-server.sh /usr/local/bin/pong-server.sh

WORKDIR /home/arduino
EXPOSE 22

CMD ["/bin/sh", "-c", "/usr/sbin/sshd -D & su arduino -c adbd & pong-server.sh"]
