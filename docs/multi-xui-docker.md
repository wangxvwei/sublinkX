# 多 VPS 3x-ui / x-ui 节点同步与 Docker 部署指南

本文档说明本 fork 的核心用法：把 SublinkX 部署成一个中心服务，从多台远程 3x-ui / x-ui VPS 中导入节点，再统一选择和生成订阅。

## 项目来源

本仓库基于以下项目二次开发：

- fork 自 [gooaclok819/sublinkX](https://github.com/gooaclok819/sublinkX)。
- 原 SublinkX 基于 [jaaksii/sublink](https://github.com/jaaksii/sublink)。
- 前端基于 [youlaitech/vue3-element-admin](https://github.com/youlaitech/vue3-element-admin)。
- 后端使用 Go、Gin、Gorm。

本 fork 主要新增：

- 多 VPS 源管理。
- SSH 密码方式同步远程 x-ui 数据库。
- 3x-ui API Token 方式同步远程面板节点。
- 自动改写远程订阅中的本地地址。
- Web 表单式节点改写规则。
- DockerHub / GHCR 自动构建发布。
- 非 Docker VPS 测试安装脚本。

## 推荐部署方式

长期使用推荐部署在 NAS 的 Docker 中。

临时测试可以部署在 VPS 上。

## Docker 部署

创建目录：

```bash
mkdir -p sublinkx/db sublinkx/logs sublinkx/template
cd sublinkx
```

创建 `docker-compose.yml`：

```yaml
services:
  sublinkx:
    image: wangxvwei/sublinkx:latest
    container_name: sublinkx
    restart: unless-stopped
    ports:
      - "8000:8000"
    environment:
      TZ: Asia/Shanghai
    volumes:
      - ./db:/app/db
      - ./logs:/app/logs
      - ./template:/app/template
```

启动：

```bash
docker compose up -d
```

访问：

```text
http://NAS_IP:8000
```

默认登录：

```text
admin / 123456
```

首次登录后请修改密码。

## 一键下载 compose 并启动

```bash
mkdir -p sublinkx && cd sublinkx
curl -fsSL https://raw.githubusercontent.com/wangxvwei/sublinkX/feature/multi-xui-sources-docker/docker-compose.multi-xui.yml -o docker-compose.yml
docker compose up -d
```

## 数据持久化

必须挂载：

```text
./db:/app/db
```

建议同时挂载：

```text
./logs:/app/logs
./template:/app/template
```

重要文件通常在：

```text
/app/db/sublink.db
/app/db/config.yaml
```

升级镜像前建议备份：

```bash
docker cp sublinkx:/app/db/. ./sublinkx-db-backup/
```

如果已经正确挂载，也可以直接备份宿主机目录：

```bash
tar -czf sublinkx-db-backup.tar.gz ./db
```

## NAS Docker UI 端口填写

如果使用飞牛、群晖、Unraid 等图形 Docker 管理器，端口映射填写：

```text
本地端口: 8000
容器端口: 8000
协议: TCP
```

如果本地 8000 被占用，可以改成本地其他端口：

```text
本地端口: 18000
容器端口: 8000
协议: TCP
```

访问时使用：

```text
http://NAS_IP:18000
```

## 非 Docker VPS 测试安装

普通安装：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/wangxvwei/sublinkX/feature/multi-xui-sources-docker/scripts/install-non-docker.sh)
```

测试安装，不影响已有 Sublink 服务：

```bash
INSTALL_DIR=/opt/sublinkx-test SERVICE_NAME=sublinkx-test SUBLINK_PORT=18000 \
bash <(curl -fsSL https://raw.githubusercontent.com/wangxvwei/sublinkX/feature/multi-xui-sources-docker/scripts/install-non-docker.sh)
```

访问：

```text
http://VPS_IP:18000
```

常用命令：

```bash
systemctl status sublinkx
systemctl restart sublinkx
journalctl -u sublinkx -n 100 --no-pager
```

如果使用测试服务名：

```bash
systemctl status sublinkx-test
systemctl restart sublinkx-test
journalctl -u sublinkx-test -n 100 --no-pager
```

## 添加 VPS 源

打开：

```text
节点管理 -> 节点列表 -> VPS 源管理
```

一个远程 VPS 添加一个源。

每个源选择一种认证方式：

```text
账号密码
API Token
```

## SSH 账号密码方式

适合 SublinkX 可以 SSH 连接远程 VPS 的情况。

示例：

```text
名称: 107
主机: 107.174.102.197
SSH 端口: 22
用户名: root
认证方式: 账号密码
密码: VPS SSH 密码
启用: 开
```

高级选项：

```text
x-ui DB: /etc/x-ui/x-ui.db
订阅 Base: https://127.0.0.1:2096
订阅路径: dingyue
分组: 107
名称前缀: [107]
删除缺失节点: 按需开启
```

说明：

- `x-ui DB` 是远程 VPS 上的面板数据库路径。
- `订阅 Base` 是在远程 VPS 本机访问订阅服务的地址。
- `订阅路径` 通常是 `dingyue`，具体以远程面板为准。
- `删除缺失节点` 开启后，如果远程节点删除，本地同步节点也会删除。

## API Token 方式

适合远程 3x-ui API 可从 SublinkX 所在机器访问的情况。

示例：

```text
名称: 144
面板地址: https://144.225.124.110:53380/xxxxx
认证方式: API Token
API Token: 远程 3x-ui 生成的 API Token
启用: 开
```

高级选项：

```text
分组: 144
名称前缀: [144]
删除缺失节点: 按需开启
```

说明：

- 面板地址要包含远程面板的完整访问路径。
- API Token 从远程 3x-ui 面板的安全设置中生成。
- 如果远程面板 HTTPS 直连不稳定，可能出现 `EOF`、`timeout`、`SSL_read`。
- 这种情况下建议改用 SSH 账号密码方式。

## 同步节点

单个源同步：

```text
VPS 源管理 -> 对应源 -> 同步
```

同步所有启用源：

```text
VPS 源管理 -> 同步全部启用源
```

同步后节点会进入普通节点池，可以在 SublinkX 中继续选择、分组和生成统一订阅。

## 节点地址自动改写

如果远程订阅返回：

```text
127.0.0.1
localhost
::1
```

SublinkX 会自动改写成该 VPS 源的公网主机。

例如远程订阅原始节点是：

```text
vless://uuid@127.0.0.1:10000?type=xhttp
```

VPS 源主机填写：

```text
192.3.233.106
```

则导入时会改成：

```text
vless://uuid@192.3.233.106:10000?type=xhttp
```

如果还需要改成 CDN 域名、443、TLS、SNI、Host 等，请使用节点改写规则。

## 节点改写规则

入口：

```text
VPS 源管理 -> 高级选项 -> 节点改写规则
```

可匹配字段：

```text
传输
协议
名称包含
```

可改写字段：

```text
地址
端口
安全
SNI
Host
Fingerprint
ALPN
Path
Flow
```

示例：把名称包含 `cdn` 的 xhttp 节点改成 CDN 公网参数：

```text
传输: xhttp
名称包含: cdn
地址: wangxvwei.top
端口: 443
安全: tls
SNI: wangxvwei.top
Host: wangxvwei.top
Fingerprint: chrome
ALPN: h2,http/1.1
Path: /api/v1/sync
```

规则说明：

- 匹配条件为空时，表示不限制该条件。
- 改写字段为空时，表示该字段不改。
- 多条规则按顺序应用。
- `名称包含` 是判断节点别名里是否包含指定文字。

## 常见问题排查

### API Token 添加成功但没有节点

检查面板 API 是否可访问：

```bash
wget -S -O- --no-check-certificate \
  --header="Authorization: Bearer <API_TOKEN>" \
  "https://<panel-host>:<panel-port>/<panel-path>/panel/api/inbounds/list"
```

如果返回 `EOF` 或 `SSL_read`，说明请求还没有正常到达可用 API，优先检查远程面板 HTTPS、端口、防火墙和路径。

### SSH 源失败

在 SublinkX 容器中测试：

```bash
nc -vz -w 5 <VPS_IP> 22
```

如果 22 端口不通，检查：

- VPS 防火墙。
- SSH 端口是否修改。
- NAS 到 VPS 的网络。
- root 密码是否正确。

### 同步后全部节点显示 0

本 fork 已修复“全部”分组初始化问题。若再次出现：

1. 刷新页面。
2. 查看浏览器控制台。
3. 查看容器日志：

```bash
docker logs sublinkx --tail 100
```

### 删除容器后数据消失

说明之前没有挂载 `/app/db`。以后必须挂载：

```text
./db:/app/db
```

如果旧容器还在，先备份：

```bash
docker cp sublinkx:/app/db/. ./sublinkx-db-backup/
```

## 镜像更新

拉取最新镜像：

```bash
docker pull wangxvwei/sublinkx:latest
docker compose up -d
```

如果 NAS 图形界面支持更新按钮，使用 `latest` 标签通常更容易识别有新版本。

如果想固定版本：

```text
wangxvwei/sublinkx:v0.1.0
```

## 开发与发布

后端测试：

```bash
go test ./...
```

前端构建：

```bash
cd webs
npm install
npm run build
```

提交：

```bash
git add .
git commit -m "docs: update usage guide"
git push origin feature/multi-xui-sources-docker
```

发布版本：

```bash
git tag v0.1.1
git push origin v0.1.1
```

GitHub Actions 会自动构建：

```text
DockerHub: wangxvwei/sublinkx
GHCR: ghcr.io/wangxvwei/sublinkx
```
