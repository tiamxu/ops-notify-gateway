# Alertmanager 接入说明

Alertmanager 通过 `channel` 指定通知路由，不直接传钉钉或飞书 webhook。

## webhook 配置示例

```yaml
receivers:
  - name: ops-notify
    webhook_configs:
      - url: http://ops-notify-gateway:8801/api/v1/alerts/alertmanager?channel=alert-prod
        send_resolved: true
        http_config:
          authorization:
            type: Bearer
            credentials: ${OPS_NOTIFY_TOKEN}
```

如果 Alertmanager 版本不支持直接配置 Bearer token，可通过内网网关补充 Header，或改用 `X-Webhook-Token` 方式调用。

## 模板

默认模板：

```text
alertmanager_dingtalk.tmpl
alertmanager_feishu.tmpl
```

可在 channel 中通过 `template` 指定模板名，不包含 `.tmpl` 后缀。

## 注意事项

- body 使用 Alertmanager 原始 webhook JSON。
- `channel` 必填。
- 不要在 Alertmanager 配置中写钉钉或飞书 webhook。