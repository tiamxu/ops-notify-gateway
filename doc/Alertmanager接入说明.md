# Alertmanager 接入说明

链路：Prometheus → Alertmanager → 通知网关 → 钉钉/飞书。先准备网关 [channel 和 Token](配置说明.md)，以下示例按环境调整地址、路径并合并到已有配置。

## 告警规则

保存为 `rules/instance-alerts.yml`：

```yaml
groups:
  - name: instance-alerts
    rules:
      - alert: InstanceDown
        expr: up == 0
        for: 2m
        labels:
          level: "3"
        annotations:
          summary: "实例 {{ $labels.instance }} 不可用"
          description: "目标 {{ $labels.job }}/{{ $labels.instance }} 连续 2 分钟抓取失败"
          recovery_description: "目标 {{ $labels.instance }} 已恢复抓取"
```

`level` 放在 `labels` 中，数字加引号，`"3"` 显示“严重”。该规则检测已有抓取目标，不检测已从配置中删除的目标。

在 `prometheus.yml` 中加载规则并指定 Alertmanager：

```yaml
rule_files:
  - rules/instance-alerts.yml
alerting:
  alertmanagers:
    - static_configs:
        - targets: ["alertmanager:9093"]
```

## Alertmanager 配置

```yaml
route:
  receiver: ops-notify
  group_by: [alertname, level]
  group_wait: 30s
  group_interval: 5m
  repeat_interval: 4h
receivers:
  - name: ops-notify
    webhook_configs:
      - url: http://ops-notify-gateway:8801/api/v1/alerts/alertmanager?channel=alert-prod&split=false
        send_resolved: true
        http_config:
          authorization:
            type: Bearer
            credentials_file: /etc/alertmanager/ops-notify-token
```

- `alert-prod` 必须是已启用的 channel；Token 文件在 Alertmanager 环境中可读，内容与网关 `auth.token` 一致，不提交仓库。普通配置不展开 `${OPS_NOTIFY_TOKEN}`。
- `send_resolved: true` 发送恢复通知；示例按等级分组，使用 `severity` 时将 `group_by` 的 `level` 改为 `severity`。
- `split=false` 合并发送；改为 `true` 逐条发送。拆分遇错停止、不回滚，Alertmanager 重试可能产生重复通知，详见[接口文档](接口文档.md#alertmanager-通知)。

## 等级与状态

| 标签值 | 中文等级 |
|---|---|
| `"0"` 或 `info` | 信息 |
| `"1"` 或 `warning` | 警告 |
| `"2"` | 一般严重 |
| `"3"` 或 `critical` | 严重 |
| `"4"` | 灾难 |

拆分时依次取单条 `labels.level`、`labels.severity`、公共 `commonLabels.level`、`commonLabels.severity` 的首个非空白值；合并时只取公共标签。值去空白、转小写后映射，其他值原样展示，均缺失为 `unknown`。

合并不会自动选最高等级；混合等级不在公共标签中时，应按等级分组或拆分。拆分后 `warning/critical` 分别显示“警告/严重”。

颜色由状态决定，与等级无关：全 firing 为红色“故障”，全 resolved 为绿色“恢复”，混合为橙色“故障”。飞书在卡片头显示等级/状态，钉钉在 Markdown 标题显示。

## @ 人与模板

默认使用 channel 的 `at`；可在规则 `annotations` 中增加 `at: "user-id-1,user-id-2"` 覆盖，飞书填用户 ID、钉钉填手机号。拆分读取各条告警，合并只读取第一条；缺失或全为空白则沿用默认列表。

默认模板为 `alertmanager_dingtalk.tmpl`、`alertmanager_feishu.tmpl`，自定义规则见[配置说明](配置说明.md)。

## 验证

在安装对应工具的环境中执行：

```bash
promtool check rules rules/instance-alerts.yml
promtool check config prometheus.yml
amtool check-config alertmanager.yml
```

检查后重新加载配置，用测试 channel 验证“故障 → 恢复”。401 检查 Token；`unknown` 检查等级标签和分组；无恢复通知检查 `send_resolved` 与告警状态。
