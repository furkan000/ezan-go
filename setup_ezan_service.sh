#!/bin/bash

SERVICE_NAME="ezan"
EXECUTABLE="./ezan"
AUDIO_DIR="$(pwd)/audio"
SERVICE_FILE="/etc/systemd/system/${SERVICE_NAME}.service"
USER_NAME=$(logname)  # Get the logged-in user
USER_HOME=$(eval echo ~$USER_NAME)

# Check if running as root
if [[ $EUID -ne 0 ]]; then
    echo "This script must be run as root (use sudo)"
    exit 1
fi

# Create systemd service file with audio support
cat <<EOF > "$SERVICE_FILE"
[Unit]
Description=Ezan Service with Audio
After=sound.target network.target

[Service]
ExecStart=$(realpath $EXECUTABLE)
WorkingDirectory=$(pwd)
Restart=always
User=$USER_NAME
Group=$USER_NAME
PAMName=login
Environment="AUDIO_DIR=${AUDIO_DIR}"
Environment="XDG_RUNTIME_DIR=/run/user/\$(id -u $USER_NAME)"
Environment="DBUS_SESSION_BUS_ADDRESS=unix:path=/run/user/\$(id -u $USER_NAME)/bus"
Environment="PULSE_SERVER=unix:/run/user/\$(id -u $USER_NAME)/pulse/native"

[Install]
WantedBy=default.target
EOF

# Reload systemd to recognize the new service
systemctl daemon-reload

# Enable the service to start on boot
systemctl enable $SERVICE_NAME

# Restart or start the service
if systemctl is-active --quiet $SERVICE_NAME; then
    echo "Restarting existing service..."
    systemctl restart $SERVICE_NAME
else
    echo "Starting new service..."
    systemctl start $SERVICE_NAME
fi

echo "Service '$SERVICE_NAME' setup successfully with audio support!"
