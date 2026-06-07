# Multi 3x-ui VPS Sync With Docker

This fork can run as a central SublinkX service and import nodes from multiple remote 3x-ui VPS servers. A source can be connected by SSH username/password or by a 3x-ui API Token.

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

## Recommended Workflow

Use the non-Docker installer on a VPS when you want to test the feature quickly:

```bash
INSTALL_DIR=/opt/sublinkx-test SERVICE_NAME=sublinkx-test SUBLINK_PORT=18000 \
bash <(curl -fsSL https://raw.githubusercontent.com/wangxvwei/sublinkX/feature/multi-xui-sources-docker/scripts/install-non-docker.sh)
```

This avoids touching an existing `/usr/local/bin/sublink` service. Open:

```text
http://VPS_IP:18000
```

For real long-term use, deploy the Docker compose file on your NAS:

```bash
mkdir -p sublinkx && cd sublinkx && \
curl -fsSL https://raw.githubusercontent.com/wangxvwei/sublinkX/feature/multi-xui-sources-docker/docker-compose.multi-xui.yml -o docker-compose.yml && \
docker compose up -d
```

For SSH password sources, the NAS only needs outbound SSH access to each 3x-ui VPS. For API Token sources, the NAS only needs outbound HTTPS access to that 3x-ui panel and its subscription port.

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

Add one source per 3x-ui VPS. Choose one connection method per source.

SSH password source:
```text
Name: SanJose
Host: 192.0.2.10
SSH port: 22
Username: root
Password: your SSH password
x-ui DB: /etc/x-ui/x-ui.db
Subscription base URL: https://127.0.0.1:2096
Subscription path: dingyue
Group name: SanJose
Name prefix: [SanJose]
```

API Token source:
```text
Name: LosAngeles
Panel URL: https://panel.example.com:54321/secret-path
API Token: your 3x-ui API token
Group name: LosAngeles
Name prefix: [LosAngeles]
```

For API Token sources, SublinkX reads the 3x-ui database backup API to detect the subscription port and path automatically. You usually do not need to fill `Subscription base URL`.

Then click `Sync`, or click `Sync all enabled sources`.

Imported nodes are written into the normal node pool, grouped by source. You can manually select any combination of nodes to build unified subscriptions. If a remote subscription returns a local loopback address such as `127.0.0.1`, `localhost`, or `::1`, SublinkX rewrites it to the source public host before saving the node.

## Remote VPS Requirements

For SSH password sources, the central SublinkX container connects to each VPS using SSH and runs a short Python script on the remote VPS. The remote VPS should have:

```text
python3
/etc/x-ui/x-ui.db
3x-ui subscription service reachable on the remote VPS itself
```

For API Token sources, the 3x-ui panel API and subscription service must be reachable from the SublinkX host. The API token must have access to inbound list and database backup endpoints.

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
