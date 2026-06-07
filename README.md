# SublinkX 多 VPS 节点聚合版

SublinkX 是一个订阅链接管理和转换工具。本仓库是在原 SublinkX 基础上做的个人二次开发版本，重点增加了“从多台 3x-ui / x-ui VPS 自动导入节点，并统一管理订阅”的能力。

## 基于哪些项目改造

本项目不是从零开始编写，来源关系如下：

- 本仓库 fork 自 [gooaclok819/sublinkX](https://github.com/gooaclok819/sublinkX)。
- 原 SublinkX 基于 [jaaksii/sublink](https://github.com/jaaksii/sublink) 二次开发。
- 前端框架基于 [youlaitech/vue3-element-admin](https://github.com/youlaitech/vue3-element-admin)。
- 后端使用 Go、Gin、Gorm。

本 fork 的新增功能主要围绕多 VPS 源同步、Docker 部署、DockerHub 自动发布、节点参数改写等场景展开。

## 适合什么场景

适合你有多台 VPS，每台 VPS 上都运行 3x-ui / x-ui，并且希望：

- 把不同 VPS 上的节点导入到一个统一面板。
- 手动选择哪些节点组成一个统一订阅。
- 在家里 NAS 上长期部署 SublinkX。
- 通过 SSH 密码或 3x-ui API Token 从远程 VPS 同步节点。
- 对导入节点额外补充或修改 TLS、SNI、Host、ALPN、Fingerprint、地址、端口、路径等参数。

## 主要功能

- 支持普通订阅管理和订阅转换。
- 支持 Docker 部署，适合 NAS 长期运行。
- 支持非 Docker 一键安装脚本，适合 VPS 临时测试。
- 支持多个远程 VPS 源。
- VPS 源支持两种接入方式：
  - SSH 账号密码。
  - 3x-ui API Token。
- 自动读取远程 3x-ui / x-ui 的节点名称、客户端 subId 和订阅链接。
- 自动把远程订阅中 `127.0.0.1`、`localhost`、`::1` 这类本地地址改写为该 VPS 源的公网地址。
- 支持按 VPS 源设置节点分组和节点名称前缀。
- 支持 Web 表单方式配置节点改写规则。
- 支持通过 GitHub Actions 自动构建并发布 DockerHub / GHCR 镜像。

## Docker 镜像

DockerHub：

```text
wangxvwei/sublinkx:latest
wangxvwei/sublinkx:v0.1.0
wangxvwei/sublinkx:feature-multi-xui-sources-docker
```

GHCR：

```text
ghcr.io/wangxvwei/sublinkx:latest
ghcr.io/wangxvwei/sublinkx:v0.1.0
ghcr.io/wangxvwei/sublinkx:feature-multi-xui-sources-docker
```

NAS 长期使用建议优先使用：

```text
wangxvwei/sublinkx:latest
```

如果想固定版本，可以使用：

```text
wangxvwei/sublinkx:v0.1.0
```

## Docker 部署

推荐在 NAS 上使用 Docker 部署。

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

默认账号密码沿用原项目：

```text
admin / 123456
```

首次登录后建议立刻修改密码。

### 重要：必须挂载数据目录

长期使用时一定要挂载：

```text
/app/db
```

否则删除容器后，SublinkX 里的节点、订阅、VPS 源配置可能会丢失。

推荐至少挂载：

```text
./db       -> /app/db
./logs     -> /app/logs
./template -> /app/template
```

## 一键 Docker 部署

也可以直接下载本仓库里的 compose 文件：

```bash
mkdir -p sublinkx && cd sublinkx
curl -fsSL https://raw.githubusercontent.com/wangxvwei/sublinkX/feature/multi-xui-sources-docker/docker-compose.multi-xui.yml -o docker-compose.yml
docker compose up -d
```

## 非 Docker VPS 测试部署

如果只是想在 VPS 上测试效果，可以使用非 Docker 安装脚本。

普通安装：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/wangxvwei/sublinkX/feature/multi-xui-sources-docker/scripts/install-non-docker.sh)
```

如果 VPS 上已经有旧 Sublink 服务，不想覆盖，可以指定测试目录和测试端口：

```bash
INSTALL_DIR=/opt/sublinkx-test SERVICE_NAME=sublinkx-test SUBLINK_PORT=18000 \
bash <(curl -fsSL https://raw.githubusercontent.com/wangxvwei/sublinkX/feature/multi-xui-sources-docker/scripts/install-non-docker.sh)
```

访问：

```text
http://VPS_IP:18000
```

## 添加远程 VPS 源

登录 SublinkX 后进入：

```text
节点管理 -> 节点列表 -> VPS 源管理
```

一个 VPS 添加一个源。每个源选择一种认证方式即可。

### 方式一：SSH 账号密码

适合 SublinkX 所在机器可以 SSH 到远程 VPS 的情况。

示例：

```text
名称: 圣何塞
主机: 107.174.102.197
SSH 端口: 22
用户名: root
认证方式: 账号密码
密码: 远程 VPS SSH 密码
x-ui DB: /etc/x-ui/x-ui.db
订阅 Base: https://127.0.0.1:2096
订阅路径: dingyue
分组: 圣何塞
名称前缀: [圣何塞]
```

工作逻辑：

1. SublinkX 通过 SSH 登录远程 VPS。
2. 在远程 VPS 上读取 3x-ui / x-ui 数据库。
3. 找到启用的 inbound 和 client。
4. 根据 client 的 subId 拉取订阅链接。
5. 将节点导入 SublinkX 的普通节点池。

远程 VPS 通常需要：

```text
python3
/etc/x-ui/x-ui.db
远程本机可访问的订阅服务
```

### 方式二：3x-ui API Token

适合远程面板 API 可以从 SublinkX 所在机器访问的情况。

示例：

```text
名称: 洛杉矶
面板地址: https://panel.example.com:54321/secret-path
认证方式: API Token
API Token: 远程 3x-ui 面板生成的 Token
分组: 洛杉矶
名称前缀: [洛杉矶]
```

API Token 方式会尝试通过面板 API 获取 inbound 和订阅相关配置。

如果 API Token 方式出现 `EOF`、`timeout`、`SSL_read` 之类问题，通常需要检查：

- 面板地址是否包含正确路径。
- 远程面板 HTTPS 是否能被外部访问。
- API Token 是否有效。
- 面板 API 是否允许访问。
- 订阅端口是否能从 SublinkX 所在机器访问。

这种情况下可以改用 SSH 账号密码方式。

## 节点分组和名称

导入节点时默认规则：

```text
分组 = VPS 源名称
节点名称 = [VPS源名称] + 原节点名称
```

可以在高级选项里手动修改：

```text
分组
名称前缀
```

例如：

```text
分组: 107
名称前缀: [107]
```

导入后可能显示为：

```text
[107]reality-vps
[107]xhttp-cdn-vps
```

## 节点改写规则

入口：

```text
VPS 源管理 -> 高级选项 -> 节点改写规则 -> 添加规则
```

可以匹配：

```text
传输
协议
名称包含
```

可以改写：

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

典型 xhttp + CDN 规则：

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

说明：

- `名称包含` 是按节点别名筛选规则。
- `Host` 是 xhttp / ws / httpupgrade 使用的请求 Host 或伪装域名。
- `SNI` 是 TLS 握手时使用的域名。
- 没有填写的字段不会改写，会保留原节点参数。

## 常见问题

### 为什么导入的节点地址是 127.0.0.1

有些 3x-ui / x-ui 节点是 Nginx 反代结构，面板订阅里会生成内部地址：

```text
127.0.0.1:10000
```

客户端不能直接使用这个地址。本 fork 已经做了自动处理：如果发现订阅链接地址是 `127.0.0.1`、`localhost`、`::1`，会自动改成 VPS 源的公网主机。

如果还需要补 TLS、SNI、Host 等参数，可以使用节点改写规则。

### 为什么 API Token 源添加成功但同步不到节点

常见原因：

- 面板地址错误。
- 面板路径缺少随机路径。
- API Token 无效。
- 远程面板 HTTPS 配置不兼容。
- 订阅服务无法从 SublinkX 所在机器访问。

建议先在 SublinkX 容器内测试：

```bash
wget -S -O- --no-check-certificate \
  --header="Authorization: Bearer <API_TOKEN>" \
  "https://<panel-url>/panel/api/inbounds/list"
```

### 为什么 NAS Docker 更新后数据没了

通常是因为没有挂载 `/app/db`。长期使用一定要挂载：

```text
./db:/app/db
```

## 本地开发

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

Docker 本地构建：

```bash
docker build -t sublinkx:local .
```

## 发布新版 Docker 镜像

本仓库已配置 GitHub Actions：

```text
.github/workflows/docker-image.yml
```

推送到以下分支或 tag 会自动构建镜像：

```text
main
feature/multi-xui-sources-docker
v*.*.*
```

GitHub Secrets 需要配置：

```text
DOCKERHUB_USERNAME
DOCKERHUB_TOKEN
```

发布新版本示例：

```bash
git add .
git commit -m "docs: update usage guide"
git push origin feature/multi-xui-sources-docker

git tag v0.1.1
git push origin v0.1.1
```

## 更多说明

详细的多 VPS / Docker 使用说明见：

```text
docs/multi-xui-docker.md
```

## License

本项目沿用原项目许可证。请同时遵守上游项目的开源协议。

