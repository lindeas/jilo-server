#!/usr/bin/env bash

###
# Jilo Server installation script
#
# Description: Installation script for Jilo Server
# Author: Yasen Pramatarov
# License: GPLv2
# Project URL: https://lindeas.com/jilo
# Year: 2025
# Version: 0.1
#
###


# Paths to init and systemd service files
SYSVINIT_SCRIPT="./jilo-server.init"
SYSTEMD_SERVICE="./jilo-server.service"
UPSTART_CONF="./jilo-server.conf"

# Function to install the SysVinit script
install_sysvinit() {

    echo "Detected SysVinit. Installing init script..."
    cp "$SYSVINIT_SCRIPT" /etc/init.d/jilo-server
    chmod +x /etc/init.d/jilo-server

    # for Debian/Ubuntu
    if command -v update-rc.d >/dev/null 2>&1; then
        update-rc.d jilo-server defaults

    # for RedHat/CentOS/Fedora
    elif command -v chkconfig >/dev/null 2>&1; then
        chkconfig --add jilo-server
    fi

    echo "SysVinit script installed."
}

# Function to install the systemd service file
install_systemd() {

    echo "Detected systemd. Installing systemd service file..."
    cp "$SYSTEMD_SERVICE" /etc/systemd/system/jilo-server.service
    systemctl daemon-reload
    systemctl enable jilo-server.service

    # compatibility with sysV
    sudo ln -s /etc/systemd/system/jilo-server.service /etc/init.d/jilo-server

    # starting the server
    systemctl start jilo-server.service

    echo "Systemd service file installed."
}

# Function to install the Upstart configuration
install_upstart() {

    echo "Detected Upstart. Installing Upstart configuration..."
    cp "$UPSTART_CONF" /etc/init/jilo-server.conf
    initctl reload-configuration

    echo "Upstart configuration installed."
}

# Detect the init system
if [[ `readlink /proc/1/exe` == */systemd ]]; then
    install_systemd

elif [[ -f /sbin/init && `/sbin/init --version 2>/dev/null` =~ upstart ]]; then
    install_upstart

else
    install_sysvinit

fi

exit 0
