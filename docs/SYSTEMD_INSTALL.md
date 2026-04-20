# Systemd Service Installation Guide

This guide explains how to install and run the Optical Probe Reader as a systemd service on a Linux system (typically Raspberry Pi).

## Prerequisites

- Raspberry Pi (or compatible Linux system) running a systemd-based distro
- Binary built: `meter` or `meter-rpi` (ARMv7)
- Root/sudo access
- (Optional) Serial device or TCP connection to meter already configured

## Installation Steps

### 1. Create Non-Root User

Create a dedicated user to run the harvester (recommended for security):

```bash
sudo useradd -r -s /usr/sbin/nologin -d /opt/optical-probe-reader optical-probe
```

Verify creation:
```bash
sudo id optical-probe
```

### 2. Create Application Directory

```bash
sudo mkdir -p /opt/optical-probe-reader
sudo mkdir -p /opt/optical-probe-reader/csv
sudo mkdir -p /opt/optical-probe-reader/csv/archive
```

Set permissions:
```bash
sudo chown -R optical-probe:optical-probe /opt/optical-probe-reader
sudo chmod 755 /opt/optical-probe-reader
sudo chmod 755 /opt/optical-probe-reader/csv
sudo chmod 755 /opt/optical-probe-reader/csv/archive
```

### 3. Create Configuration Directory

```bash
sudo mkdir -p /etc/optical-probe-reader
sudo chown root:optical-probe /etc/optical-probe-reader
sudo chmod 750 /etc/optical-probe-reader
```

### 4. Copy Binary

Copy the compiled binary to the application directory:

```bash
sudo cp meter /opt/optical-probe-reader/meter
# OR for Raspberry Pi (ARMv7):
sudo cp meter-rpi /opt/optical-probe-reader/meter

# Set executable permission
sudo chmod 755 /opt/optical-probe-reader/meter
```

### 5. Configure Application

Copy your config file to the system configuration directory:

```bash
sudo cp config.csv.yaml /etc/optical-probe-reader/config.yaml
sudo chown root:optical-probe /etc/optical-probe-reader/config.yaml
sudo chmod 640 /etc/optical-probe-reader/config.yaml
```

**Edit the config** to match your environment:

```bash
sudo nano /etc/optical-probe-reader/config.yaml
```

Update transport type (serial/tcp), device paths, IP addresses, and collection interval as needed.

### 6. Install Systemd Service

Copy the service file:

```bash
sudo cp optical-probe-reader.service /etc/systemd/system/
```

Reload systemd daemon:

```bash
sudo systemctl daemon-reload
```

Verify the service file is recognized:

```bash
sudo systemctl list-unit-files | grep optical-probe
```

### 7. Enable and Start Service

Enable the service to start at boot:

```bash
sudo systemctl enable optical-probe-reader.service
```

Start the service immediately:

```bash
sudo systemctl start optical-probe-reader.service
```

### 8. Verify Status

Check if the service is running:

```bash
sudo systemctl status optical-probe-reader.service
```

Expected output:
```
● optical-probe-reader.service - Optical Probe Reader - IEC 62056-21 Meter Data Harvester
     Loaded: loaded (/etc/systemd/system/optical-probe-reader.service; enabled; vendor preset: disabled)
     Active: active (running) since Sat 2026-04-19 21:00:00 BST; 5s ago
   Main PID: 12345 (meter)
      Tasks: 5 (limit: 1024)
     Memory: 8.2M
     CGroup: /system.slice/optical-probe-reader.service
             └─12345 /opt/optical-probe-reader/meter harvest -c /etc/optical-probe-reader/config.yaml
```

## Viewing Logs

### Real-time logs:

```bash
sudo journalctl -u optical-probe-reader.service -f
```

### Last 50 lines:

```bash
sudo journalctl -u optical-probe-reader.service -n 50
```

### Logs since last boot:

```bash
sudo journalctl -u optical-probe-reader.service -b
```

### Logs with timestamps and full details:

```bash
sudo journalctl -u optical-probe-reader.service --no-pager -o long-iso
```

## Managing the Service

### Stop the service:

```bash
sudo systemctl stop optical-probe-reader.service
```

### Restart the service:

```bash
sudo systemctl restart optical-probe-reader.service
```

### View service configuration:

```bash
sudo systemctl cat optical-probe-reader.service
```

### Check service dependencies:

```bash
systemctl list-dependencies optical-probe-reader.service
```

## Troubleshooting

### Service fails to start

Check the status and logs:

```bash
sudo systemctl status optical-probe-reader.service
sudo journalctl -u optical-probe-reader.service -n 100
```

Common issues:
- **Config file not found**: Verify `/etc/optical-probe-reader/config.yaml` exists and is readable
- **Binary not found**: Check `/opt/optical-probe-reader/meter` exists and is executable
- **Permission denied**: Check file ownership (`optical-probe:optical-probe`) and directory permissions
- **Device not found**: Verify serial device (e.g., `/dev/ttyUSB0`) exists and user has permissions

### Adding user to serial group (if using serial transport):

```bash
sudo usermod -a -G dialout optical-probe
# Restart service after group change
sudo systemctl restart optical-probe-reader.service
```

### Viewing CSV output directory:

```bash
ls -lh /opt/optical-probe-reader/csv/
ls -lh /opt/optical-probe-reader/csv/archive/
```

### Manual test (before installing as service):

```bash
sudo -u optical-probe /opt/optical-probe-reader/meter harvest -c /etc/optical-probe-reader/config.yaml
```

## Updating the Application

When you rebuild the binary on your development machine:

1. Stop the service: `sudo systemctl stop optical-probe-reader.service`
2. Copy the new binary: `sudo cp meter /opt/optical-probe-reader/meter`
3. Restart the service: `sudo systemctl start optical-probe-reader.service`

Or as a one-liner (with downtime):

```bash
sudo cp meter /opt/optical-probe-reader/meter && sudo systemctl restart optical-probe-reader.service
```

## Boot Sequence

The service is configured to start in the `multi-user.target` after the network is online. Boot order:

1. System starts and initializes basic services
2. Network interface comes up (`network-online.target`)
3. Multi-user level is reached (`multi-user.target`)
4. **Optical Probe Reader service starts** (if enabled)
5. System fully boot complete

This ensures:
- Serial device enumeration is complete (for `/dev/ttyUSB0` detection)
- Network is available (for TCP meter connections)
- All filesystem mounts are ready (for CSV directory access)

## Uninstall

To remove the service:

```bash
sudo systemctl stop optical-probe-reader.service
sudo systemctl disable optical-probe-reader.service
sudo rm /etc/systemd/system/optical-probe-reader.service
sudo systemctl daemon-reload

# Optional: remove application files
sudo rm -rf /opt/optical-probe-reader
sudo rm -rf /etc/optical-probe-reader

# Optional: remove user account
sudo userdel optical-probe
```

## Security Notes

The service is configured with the following security hardening:
- Runs as non-root user (`optical-probe`)
- No new privileges (`NoNewPrivileges=true`)
- Filesystem is mounted read-only except for CSV directories (`ProtectSystem=strict`)
- Home directory is hidden (`ProtectHome=yes`)
- Restart limit prevents rapid restart loops (5 restarts in 300 seconds)

## Next Steps

- Monitor CSV output in `/opt/optical-probe-reader/csv/`
- Set up log aggregation (e.g., ELK stack) if needed
- Integrate with monitoring systems (Prometheus, Grafana, etc.)
- Consider backup strategy for CSV archive directory
