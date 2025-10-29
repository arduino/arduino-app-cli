# 1. Use Debian base
FROM debian:trixie

ENV DEBIAN_FRONTEND=noninteractive

# 2. Install necessary tools
RUN apt update && apt install -y \
    dpkg-dev \
    apt-utils \
    adduser \
    && rm -rf /var/lib/apt/lists/*

# 3. Symlink addgroup for minimal system scripts
RUN ln -s /usr/sbin/addgroup /usr/bin/addgroup || true

# 4. Build args for parameterization
ARG OLD_PACKAGE_PATH=build/old_package
ARG NEW_PACKAGE_PATH=build
ARG APP_PACKAGE_NAME=arduino-app-cli
ARG ROUTER_PACKAGE_NAME=arduino-router
ARG ARCH=arm64
ARG VERSION=0.6.3

# 5. Copy packages dynamically
COPY ${OLD_PACKAGE_PATH}/${APP_PACKAGE_NAME}*.deb /tmp/old_app.deb
COPY ${NEW_PACKAGE_PATH}/${APP_PACKAGE_NAME}*.deb /tmp/new_app.deb
COPY ${NEW_PACKAGE_PATH}/${ROUTER_PACKAGE_NAME}*.deb /tmp/new_router.deb

# 6. Install old package + router dependency
RUN apt update && apt install -y \
    /tmp/old_app.deb \
    /tmp/new_router.deb \
    && rm /tmp/old_app.deb

# 7. Setup local APT repo with new packages
RUN mkdir -p /var/www/html/myrepo/dists/trixie/main/binary-${ARCH}

# Rename new packages to match their real package/version/arch
RUN mv /tmp/new_app.deb /var/www/html/myrepo/dists/trixie/main/binary-${ARCH}/${APP_PACKAGE_NAME}_${VERSION}_${ARCH}.deb
RUN mv /tmp/new_router.deb /var/www/html/myrepo/dists/trixie/main/binary-${ARCH}/${ROUTER_PACKAGE_NAME}_0.6.2-1_${ARCH}.deb

# 8. Generate Packages.gz metadata
WORKDIR /var/www/html/myrepo
RUN dpkg-scanpackages dists/trixie/main/binary-arm64 /dev/null | gzip -9c > dists/trixie/main/binary-arm64/Packages.gz
WORKDIR /

# 9. Configure local APT repo
RUN echo "deb [trusted=yes arch=${ARCH}] file:/var/www/html/myrepo trixie main" \
    > /etc/apt/sources.list.d/my-mock-repo.list

# 10. Fix home dir for arduino user (optional)
RUN usermod -s /bin/bash arduino || true
RUN mkdir -p /home/arduino && chown -R arduino:arduino /home/arduino

# 11. Entrypoint: show upgrade availability
RUN echo '#!/bin/bash\n\
set -e\n\
echo "--- Updating APT ---"\n\
apt update\n\
echo "--- Installed version ---"\n\
dpkg -l | grep ${APP_PACKAGE_NAME} || true\n\
echo "--- Upgrade candidate ---"\n\
apt-cache policy ${APP_PACKAGE_NAME}\n\
echo "--- Available upgrades ---"\n\
apt list --upgradable | grep ${APP_PACKAGE_NAME} || echo "No upgrade found"\n\
echo "--- Simulating upgrade ---"\n\
apt upgrade --simulate\n\
' > /entrypoint.sh

RUN chmod +x /entrypoint.sh
CMD ["/entrypoint.sh"]
