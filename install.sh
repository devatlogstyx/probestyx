#!/bin/bash

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
GITHUB_REPO="devatlogstyx/probestyx"
INSTALL_DIR="/opt/probestyx"
SERVICE_USER="probestyx"
BINARY_NAME="probestyx"
CONFIG_FILE="config.yaml"

echo -e "${GREEN}Probestyx Installation Script${NC}"
echo "=============================="

# Check for config file parameter
if [ -z "$1" ]; then
    echo -e "${RED}Error: Config file path is required${NC}"
    echo ""
    echo "Usage:"
    echo "  wget -O - https://raw.githubusercontent.com/.../install.sh | sudo bash -s /path/to/config.yaml"
    echo "  curl -fsSL https://raw.githubusercontent.com/.../install.sh | sudo bash -s /path/to/config.yaml"
    echo ""
    echo "Example:"
    echo "  wget -O - https://raw.githubusercontent.com/.../install.sh | sudo bash -s ./config.yaml"
    exit 1
fi

CONFIG_SOURCE="$1"

# Check if config file exists
if [ ! -f "$CONFIG_SOURCE" ]; then
    echo -e "${RED}Error: Config file not found: $CONFIG_SOURCE${NC}"
    exit 1
fi

echo -e "${GREEN}Using config file: $CONFIG_SOURCE${NC}"

# Check if running as root
if [ "$EUID" -ne 0 ]; then 
    echo -e "${RED}Error: Please run as root (use sudo)${NC}"
    exit 1
fi

# Detect OS and Architecture
OS=""
ARCH=$(uname -m)

if [[ "$OSTYPE" == "linux-gnu"* ]]; then
    OS="linux"
elif [[ "$OSTYPE" == "darwin"* ]]; then
    OS="darwin"
else
    echo -e "${RED}Error: Unsupported OS: $OSTYPE${NC}"
    exit 1
fi

# Map architecture names
case $ARCH in
    x86_64)
        ARCH="amd64"
        ;;
    aarch64|arm64)
        ARCH="arm64"
        ;;
    armv7l)
        ARCH="arm"
        ;;
    *)
        echo -e "${RED}Error: Unsupported architecture: $ARCH${NC}"
        exit 1
        ;;
esac

echo -e "${YELLOW}Detected OS: $OS${NC}"
echo -e "${YELLOW}Detected Architecture: $ARCH${NC}"

# Get latest release version
echo "Fetching latest release..."
LATEST_VERSION=$(curl -s "https://api.github.com/repos/$GITHUB_REPO/releases/latest" | grep '"tag_name"' | cut -d'"' -f4)

if [ -z "$LATEST_VERSION" ]; then
    echo -e "${RED}Error: Could not fetch latest version${NC}"
    exit 1
fi

echo -e "${GREEN}Latest version: $LATEST_VERSION${NC}"

# Download binary
BINARY_URL="https://github.com/$GITHUB_REPO/releases/download/latest/probestyx-$OS-$ARCH"
echo "Downloading from $BINARY_URL..."

TEMP_BINARY="/tmp/probestyx-download"
if ! curl -L -f -o "$TEMP_BINARY" "$BINARY_URL"; then
    echo -e "${RED}Error: Failed to download binary. The URL might be incorrect or the release doesn't exist.${NC}"
    exit 1
fi

if ! file "$TEMP_BINARY" | grep -qE 'ELF|Mach-O'; then
    echo -e "${RED}Error: Downloaded file is not a valid binary! Check the GitHub release assets.${NC}"
    rm "$TEMP_BINARY"
    exit 1
fi

# Check if already installed
if [ -f "$INSTALL_DIR/$BINARY_NAME" ]; then
    echo -e "${YELLOW}Probestyx is already installed. Upgrading...${NC}"
    UPGRADE=true
else
    UPGRADE=false
fi

# Create user (Linux only)
if [ "$OS" == "linux" ]; then
    if ! id "$SERVICE_USER" &>/dev/null; then
        echo "Creating user $SERVICE_USER..."
        useradd -r -s /bin/false "$SERVICE_USER"
    else
        echo "User $SERVICE_USER already exists"
    fi
fi

# Create installation directory
echo "Creating directory $INSTALL_DIR..."
mkdir -p "$INSTALL_DIR"

# Install binary
echo "Installing binary..."
mv "$TEMP_BINARY" "$INSTALL_DIR/$BINARY_NAME"
chmod +x "$INSTALL_DIR/$BINARY_NAME"

# Install config
if [ "$UPGRADE" = true ]; then
    echo -e "${YELLOW}Upgrading: Replacing config file with your new config...${NC}"
    echo -e "${YELLOW}(Your old config will be backed up)${NC}"
    if [ -f "$INSTALL_DIR/$CONFIG_FILE" ]; then
        cp "$INSTALL_DIR/$CONFIG_FILE" "$INSTALL_DIR/$CONFIG_FILE.backup.$(date +%s)"
    fi
fi

echo "Copying config file..."
cp "$CONFIG_SOURCE" "$INSTALL_DIR/$CONFIG_FILE"

# Set ownership
if [ "$OS" == "linux" ]; then
    chown -R "$SERVICE_USER:$SERVICE_USER" "$INSTALL_DIR"
elif [ "$OS" == "darwin" ]; then
    chown -R "$(whoami):staff" "$INSTALL_DIR"
fi

# Install as service
if [ "$OS" == "linux" ]; then
    if command -v systemctl &> /dev/null; then
        if [ "$UPGRADE" = true ]; then
            echo "Restarting service after upgrade..."
            systemctl restart probestyx
            echo -e "${GREEN}✓ Service upgraded and restarted${NC}"
        else
            echo "Installing systemd service..."
            
            cat > /etc/systemd/system/probestyx.service <<EOF
[Unit]
Description=Probestyx Metrics Collection Service
After=network.target
Wants=network-online.target

[Service]
Type=simple
User=$SERVICE_USER
Group=$SERVICE_USER
WorkingDirectory=$INSTALL_DIR
ExecStart=$INSTALL_DIR/$BINARY_NAME $INSTALL_DIR/$CONFIG_FILE
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal
SyslogIdentifier=probestyx

# Security hardening
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=$INSTALL_DIR

[Install]
WantedBy=multi-user.target
EOF

        systemctl daemon-reload
        systemctl enable probestyx
        systemctl start probestyx
        
        echo -e "${GREEN}✓ Service installed and started${NC}"
        fi
        
        echo ""
        echo "Manage the service with:"
        echo "  sudo systemctl status probestyx"
        echo "  sudo systemctl restart probestyx"
        echo "  sudo systemctl stop probestyx"
        echo "  sudo journalctl -u probestyx -f"
        
    else
        echo -e "${YELLOW}Warning: systemd not found${NC}"
        echo "Run manually: sudo $INSTALL_DIR/$BINARY_NAME $INSTALL_DIR/$CONFIG_FILE"
    fi
    
elif [ "$OS" == "darwin" ]; then
    if [ "$UPGRADE" = true ]; then
        echo "Restarting service after upgrade..."
        launchctl unload /Library/LaunchDaemons/com.probestyx.plist 2>/dev/null || true
        launchctl load /Library/LaunchDaemons/com.probestyx.plist
        echo -e "${GREEN}✓ Service upgraded and restarted${NC}"
    else
        echo "Installing launchd service..."
        
        cat > /Library/LaunchDaemons/com.probestyx.plist <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.probestyx</string>
    <key>ProgramArguments</key>
    <array>
        <string>$INSTALL_DIR/$BINARY_NAME</string>
        <string>$INSTALL_DIR/$CONFIG_FILE</string>
    </array>
    <key>WorkingDirectory</key>
    <string>$INSTALL_DIR</string>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>/var/log/probestyx.log</string>
    <key>StandardErrorPath</key>
    <string>/var/log/probestyx.log</string>
</dict>
</plist>
EOF

    launchctl load /Library/LaunchDaemons/com.probestyx.plist
    
    echo -e "${GREEN}✓ Service installed and started${NC}"
    fi
    
    echo ""
    echo "Manage the service with:"
    echo "  sudo launchctl list | grep probestyx"
    echo "  sudo launchctl stop com.probestyx"
    echo "  sudo launchctl start com.probestyx"
    echo "  tail -f /var/log/probestyx.log"
fi

echo ""
if [ "$UPGRADE" = true ]; then
    echo -e "${GREEN}Upgrade complete!${NC}"
else
    echo -e "${GREEN}Installation complete!${NC}"
fi
echo "Installed version: $LATEST_VERSION"
echo "Config location: $INSTALL_DIR/$CONFIG_FILE"
echo "Service is running on http://localhost:9100"
echo "Test with: curl http://localhost:9100/metrics"