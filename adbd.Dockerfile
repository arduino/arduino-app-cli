FROM debian:trixie

RUN apt-get update \
    && apt-get install -y --no-install-recommends adbd file openssh-server \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/*

RUN useradd -m -s /bin/bash arduino && \
    echo "arduino:arduino" | chpasswd
RUN mkdir /home/arduino/arduino-apps && \
    chown arduino:arduino /home/arduino/arduino-apps

WORKDIR /home/arduino
EXPOSE 22

CMD ["/bin/sh", "-c", "/usr/sbin/sshd -D & adbd"]
