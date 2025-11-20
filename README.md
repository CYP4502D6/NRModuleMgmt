
# NR Module Monitor

A Go application for monitoring Quectel's 5G NR celluar network modules via AT commands, featuring SMS management and Discord integration.
Q
## Features
- Retrieve module information (name, CPU temp, SIM status)
- Query network details (APN, IP addresses, cell ID, data usage)
- Fetch signal metrics (RSRP, RSRQ, SINR for LTE/5G)
- SMS management (receive, send, delete with database storage)
- Discord bot integration for remote control and notifications

## Configuration
Create a `config/config.yaml` file with the following structure:
```yaml
serial:
  is_local: true
  port: "/dev/ttyUSB2"
  baud_rate: 9600
  remote_api: ""

sms:
  db_path: "sms.db"
  check_interval: 30s

discord:
  bot_token: "your_bot_token"
  channel_id: "your_channel_id"
```

## Usage

Use Discord commands (prefix `!`) in your configured channel:
   - `!info module` - Query module information
   - `!info network` - Query network information
   - `!info signal` - Query signal information
   - `!sms send <phone> <message>` - Send SMS
   - `!sms count` - Get SMS count
   - `!sms get <id>` - Retrieve specific SMS
   - `!sms list <start> <end>` - List SMS by ID range
   - `!check` - Trigger manual SMS check
