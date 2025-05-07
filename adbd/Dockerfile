FROM debian:trixie

RUN apt-get update \
    && apt-get install -y --no-install-recommends adbd file \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/*

RUN mkdir /apps
WORKDIR /apps

CMD ["adbd"]
