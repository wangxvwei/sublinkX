# Third-party notices / 第三方项目说明

This repository builds on open-source projects and uses public data services.
The main projects and their roles are listed below. Dependency manifests remain
the authoritative lists of packages used by the application.

本项目在开源项目基础上继续开发，并按需使用公开数据服务。感谢所有上游作者、
贡献者与维护者。主要依赖清单见 [go.mod](go.mod) 和
[webs/package.json](webs/package.json)；各组件分别遵循其自身许可证。

## Project origins / 项目来源

| Project | Role |
| --- | --- |
| [gooaclok819/sublinkX](https://github.com/gooaclok819/sublinkX) | Direct upstream / 本仓库直接上游 |
| [jaaksii/sublink](https://github.com/jaaksii/sublink) | Basis of the original sublinkX / 原 sublinkX 的基础项目 |
| [youlaitech/vue3-element-admin](https://github.com/youlaitech/vue3-element-admin) | Admin frontend foundation / 管理端界面基础 |
| [PanJiaChen/vue-element-admin](https://github.com/PanJiaChen/vue-element-admin) | Components and assets retained through the frontend template / 前端模板沿用的组件与资源 |

The repository's [MIT license](LICENSE) and the frontend's upstream notices are
retained. The frontend notices are available at:

- [webs/licenses/vue3-element-admin/LICENSE](webs/licenses/vue3-element-admin/LICENSE)
- [webs/licenses/vue-element-admin/LICENSE](webs/licenses/vue-element-admin/LICENSE)

## CloudflareSpeedTest

The Docker image includes the unmodified CloudflareSpeedTest v2.3.5 command-line
program by XIU2, invoked as a separate process for CDN measurements.

Project: https://github.com/XIU2/CloudflareSpeedTest/tree/v2.3.5

Its GNU GPL version 3 license is included at
`/usr/share/doc/cloudflare-speed-test/LICENSE`. Its corresponding source,
including vendored Go dependencies and their licenses, is included in the same
image at `/usr/share/doc/cloudflare-speed-test/source.tar.gz`.

Extract that archive and build with Go 1.22.2:

```sh
CGO_ENABLED=0 go build -mod=vendor -trimpath -ldflags="-s -w -X main.version=v2.3.5" -o cfst .
```

No modifications have been made to CloudflareSpeedTest. The exact image build
steps are recorded in this repository's Dockerfile.

本项目通过独立命令行进程调用未修改的 CloudflareSpeedTest v2.3.5。上述镜像路径
包含其 GNU GPL v3 许可证、对应源码及其依赖许可证，可用于查看与重新构建。

## Frameworks and components / 框架与组件

- Backend: Go, [Gin](https://github.com/gin-gonic/gin),
  [GORM](https://github.com/go-gorm/gorm),
  [glebarez/sqlite](https://github.com/glebarez/sqlite),
  [Go SSH](https://pkg.go.dev/golang.org/x/crypto/ssh),
  [base64Captcha](https://github.com/mojocn/base64Captcha),
  [jwt-go](https://github.com/dgrijalva/jwt-go), and
  [go-yaml](https://github.com/go-yaml/yaml).
- Frontend: [Vue](https://github.com/vuejs/core),
  [Vite](https://github.com/vitejs/vite),
  [Element Plus](https://github.com/element-plus/element-plus),
  [Pinia](https://github.com/vuejs/pinia),
  [Vue Router](https://github.com/vuejs/router),
  [Axios](https://github.com/axios/axios),
  [Apache ECharts](https://github.com/apache/echarts),
  [VueDraggablePlus](https://github.com/Alfred-Skyblue/vue-draggable-plus), and
  [qrcode.vue](https://github.com/scopewu/qrcode.vue).

## Public data services / 公开数据服务

| Service | Usage |
| --- | --- |
| [麒麟优选](https://api.uouin.com/cloudflare.html) / [BestCF mirror](https://bestcf.pages.dev/uouin/all.txt) | Public ISP recommendations, fetched from the BestCF mirror / 异地线路推荐的公开数据 |
| [PConline IP lookup](https://whois.pconline.com.cn/) | Target IP location and ISP lookup / 目标 IP 地区与运营商查询 |
| [ipapi.is](https://ipapi.is/) | Fallback ASN information / 辅助识别运营商的 ASN 信息 |
| [Cloudflare IP ranges](https://www.cloudflare.com/ips/) | Validate recommended addresses / 推荐地址归属校验 |

These are external services, not bundled libraries. Their data, usage limits,
and availability are controlled by their respective providers.

上述服务属于外部数据来源，不属于镜像内捆绑的代码依赖。相关数据与服务限制
以各提供方的说明为准；使用这些服务不表示提供方对本项目的认可或背书。
