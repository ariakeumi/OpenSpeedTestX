#!/bin/sh

set -eu

PACKAGE_NAME="${PACKAGE_NAME:-openspeedtestx}"
DISPLAY_NAME="${DISPLAY_NAME:-OpenSpeedTestX}"
PACKAGE_VERSION="${PACKAGE_VERSION:-1.0.0-0001}"
PACKAGE_ARCH="${PACKAGE_ARCH:-x86_64}"
PACKAGE_PORT="${PACKAGE_PORT:-3000}"
PACKAGE_DESCRIPTION="${PACKAGE_DESCRIPTION:-Self-hosted network speed test with persistent history for Synology DSM.}"
PACKAGE_MAINTAINER="${PACKAGE_MAINTAINER:-q000q000}"
PACKAGE_DISTRIBUTOR="${PACKAGE_DISTRIBUTOR:-q000q000}"
PACKAGE_OS_MIN_VER="${PACKAGE_OS_MIN_VER:-7.0-40000}"

cat <<EOF
package="$PACKAGE_NAME"
displayname="$DISPLAY_NAME"
version="$PACKAGE_VERSION"
os_min_ver="$PACKAGE_OS_MIN_VER"
description="$PACKAGE_DESCRIPTION"
maintainer="$PACKAGE_MAINTAINER"
distributor="$PACKAGE_DISTRIBUTOR"
arch="$PACKAGE_ARCH"
adminprotocol="http"
adminport="$PACKAGE_PORT"
adminurl=""
checkport="yes"
precheckstartstop="yes"
ctl_stop="yes"
ctl_uninstall="yes"
support_move="yes"
silent_install="no"
silent_upgrade="no"
silent_uninstall="no"
EOF
