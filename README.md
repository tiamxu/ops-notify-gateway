# ops-notify-gateway

统一运维通知网关，将 Jenkins、Alertmanager 和通用事件按 `channel` 发送到钉钉或飞书。

基于 Go + Gin，使用 YAML 配置和 Token 鉴权。渠道支持本地 YAML 或平台定时拉取，通过 `channel_source.mode` 切换。钉钉和飞书均支持可选加签，飞书发送卡片消息。当前同步发送，无数据库、管理后台、队列、自动重试或通知历史。

## 快速开始

需要 Go 1.23.4+，从项目根目录执行。

**1. 准备配置**（已有配置不要覆盖）

```bash
cp config/config.yaml.example config/config.yaml
```

编辑 `auth.token` 和目标 channel 的 webhook；钉钉或飞书启用加签时填写对应 secret。示例凭据均为占位值，字段说明见[配置文档](doc/配置说明.md)。

使用平台渠道时，将 `channel_source.mode` 改为 `platform`，配置读取接口 URL、独立 Token、恢复文件路径。本机测试允许 `http://localhost:<端口>`、`http://127.0.0.1:<端口>`、`http://[::1]:<端口>`，并填写完整接口路径；其他地址要求 HTTPS。本地 `channels` 可以保留，但平台模式不使用它。Jenkins、Alertmanager 始终直连网关，平台只管理和提供配置。

**2. 启动服务**

```bash
go run .
```

默认监听 `:8801`，读取 `config/config.yaml`；指定其他配置可用 `CONFIG_FILE=/path/to/config.yaml go run .`。配置修改后重启生效。

**3. 检查服务**

```bash
curl --fail http://127.0.0.1:8801/ping
```

返回 `data.message=pong` 表示 HTTP 服务正常，不代表机器人发送成功。

`GET /ready` 检查是否已有可用渠道快照，并提供同步和持久化状态；无有效快照返回 503。平台模式部署需挂载持久卷保存恢复文件。

## 发送通知

| 来源 | POST 接口 | 接入说明 |
|---|---|---|
| Jenkins | `/api/v1/notifications/jenkins` | [Pipeline 示例](doc/Jenkins接入说明.md) |
| Alertmanager | `/api/v1/alerts/alertmanager?channel=alert-prod` | [告警规则与等级](doc/Alertmanager接入说明.md) |
| Generic | `/api/v1/notifications/generic` | [请求示例](doc/接口文档.md#generic) |

通知接口默认需要 `Authorization: Bearer <token>` 或 `X-Webhook-Token: <token>`。

配置好测试机器人后，可发送一条 Jenkins 通知：

```bash
TOKEN='<与配置一致的 Token>' TEST_CASE=jenkins JENKINS_CHANNEL=jenkins-test sh scripts/test_api.sh
```

脚本支持 `basic/generic/jenkins/alertmanager/all`，默认 `basic` 不发通知。Generic 默认使用 `ops-test`，需先按[配置说明](doc/配置说明.md#channel)添加；发送模式会调用真实机器人。

## 编译与部署

```bash
go build -o ops-notify .
./ops-notify
```

部署需携带私有配置和 `templates/`。模板从进程工作目录读取，不随 `CONFIG_FILE` 所在目录改变；真实配置不要提交仓库。

## 开发与文档

验证命令：`go test ./...`、`go vet ./...`、`git diff --check`。

- [配置说明](doc/配置说明.md)：channel、模板、鉴权。
- [接口文档](doc/接口文档.md)：字段、响应、错误码和发送语义。
- [开发规范](doc/项目开发规范.md)：分层、安全和测试要求。
