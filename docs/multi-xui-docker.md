# Multi 3x-ui VPS Sync With Docker

This fork can run as a central SublinkX service and import nodes from multiple remote 3x-ui VPS servers over SSH.

## One-command Deploy

Docker:

```bash
mkdir -p sublinkx && cd sublinkx && \
curl -fsSL https://raw.githubusercontent.com/wangxvwei/sublinkX/feature/multi-xui-sources-docker/docker-compose.multi-xui.yml -o docker-compose.yml && \
docker compose up -d
```

Non-Docker systemd install:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/wangxvwei/sublinkX/feature/multi-xui-sources-docker/scripts/install-non-docker.sh)
```

Open:

```text
http://SERVER_IP:8000
```

Default login is unchanged from upstream:

```text
admin / 123456
```

Change it after first login.

## Add Remote VPS Sources

Go to the node management page and open:

```text
VPS Source Manager
```

Add one source per 3x-ui VPS:

```text
Name: SanJose
Host: 192.0.2.10
SSH port: 22
Username: root
Password or private key: your SSH credential
x-ui DB: /etc/x-ui/x-ui.db
Subscription base URL: https://127.0.0.1:2096
Subscription path: dingyue
Group name: SanJose
Name prefix: [SanJose]
```

Then click `Sync`, or click `Sync all enabled sources`.

Imported nodes are written into the normal node pool, grouped by source. You can manually select any combination of nodes to build unified subscriptions.

## Remote VPS Requirements

The central SublinkX container connects to each VPS using SSH and runs a short Python script on the remote VPS. The remote VPS should have:

```text
python3
/etc/x-ui/x-ui.db
3x-ui subscription service reachable on the remote VPS itself
```

The 3x-ui panel does not need to expose its admin API publicly.

## Persistent Data

The compose file persists:

```text
./db       -> /app/db
./logs     -> /app/logs
./template -> /app/template
```

Back up `./db/sublink.db` before upgrades.

## Optional Source Preload

You can preload VPS sources by setting `SUBLINK_XUI_SOURCES_JSON` in `docker-compose.yml`. Sources can also be created from the web UI, which is usually safer.
