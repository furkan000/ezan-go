#!/bin/bash

SERVICE_NAME="ezan"
SERVICE_FILE="/etc/systemd/system/${SERVICE_NAME}.service"

# Check if running as root
if [[ $EUID -ne 0 ]]; then
    echo "This script must be run as root (use sudo)"
    exit 1
fi

# Stop the service if it's running
if systemctl is-active --quiet $SERVICE_NAME; then
    echo "Stopping service..."
    systemctl stop $SERVICE_NAME
fi

# Disable the service so it doesn’t start on boot
if systemctl is-enabled --quiet $SERVICE_NAME; then
    echo "Disabling service..."
    systemctl disable $SERVICE_NAME
fi

# Remove the service file
if [[ -f "$SERVICE_FILE" ]]; then
    echo "Removing service file..."
    rm -f "$SERVICE_FILE"
else
    echo "Service file does not exist, skipping..."
fi

# Reload systemd to apply changes
systemctl daemon-reload

echo "Service '$SERVICE_NAME' has been uninstalled successfully!"
