# ops-notify-gateway

统一运维通知网关，接收 Jenkins、Alertmanager 等运维事件，按 `channel` 路由到钉钉、飞书等通知平台。

## 第一版能力

- Alertmanager 告警通知
- Jenkins 构建通知
- Generic 通用通知
- 钉钉机器人通知，支持加签
- 飞书机器人通知，第一版发送卡片消息
- Token 鉴权
- YAML 配置

第一版不使用数据库，不提供管理后台，不实现异步队列、失败重试和通知记录查询。

## 启动

复制配置示例并按实际环境修改配置文件：

```bash
cp config/config.yaml.example config/config.yaml
```

运行：

```bash
go run .
```

指定配置文件：

```bash
CONFIG_FILE=/path/to/config.yaml go run .
```

## 接口

```text
GET  /ping
POST /api/v1/alerts/alertmanager?channel=alert-prod
POST /api/v1/notifications/jenkins
POST /api/v1/notifications/generic
```

鉴权 Header 二选一：

```text
Authorization: Bearer <token>
X-Webhook-Token: <token>
```

详细说明见 `doc/接口文档.md`、`doc/Jenkins接入说明.md`、`doc/Alertmanager接入说明.md`。

## 验证

```bash
gofmt -w main.go api config middleware pkg routes service types
go test ./...
go vet ./...
```
