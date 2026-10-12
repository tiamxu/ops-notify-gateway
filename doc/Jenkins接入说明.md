# Jenkins 接入说明

在已有 Declarative Pipeline 中接入：`post` 放入 `pipeline`，辅助函数放在外部。需要可用工作区、curl、网关 channel，以及 ID 为 `ops-notify-token` 的 Secret text 凭据。

设置环境变量：

```text
OPS_NOTIFY_URL=http://ops-notify-gateway:8801
OPS_NOTIFY_CHANNEL=jenkins-test
```

## Pipeline 示例

```groovy
post {
    success { script { notifyBuild('SUCCESS') } }
    failure { script { notifyBuild('FAILURE') } }
    aborted { script { notifyBuild('ABORTED') } }
    unstable { script { notifyBuild('UNSTABLE') } }
}

def notifyBuild(String status) {
    def payload = groovy.json.JsonOutput.toJson([
        channel: env.OPS_NOTIFY_CHANNEL ?: 'jenkins-test',
        status: status,
        jobName: env.JOB_NAME,
        buildNumber: env.BUILD_NUMBER,
        buildUrl: env.BUILD_URL,
        services: (params.SERVICES ?: '').split('[,\\s]+').findAll { it },
        branch: params.BRANCH ?: '',
        deployEnv: params.DEPLOY_ENV ?: '',
        imageRefs: (params.IMAGE_REFS ?: '').split('[,\\s]+').findAll { it },
        packageType: params.PACKAGE_TYPE ?: '',
        deploy: params.DEPLOY?.toBoolean(),
        deployType: params.DEPLOY_TYPE ?: '',
        triggerUser: currentBuild.getBuildCauses('hudson.model.Cause$UserIdCause')?.getAt(0)?.userId ?: 'unknown'
    ])

    def payloadFile = "${pwd(tmp: true)}/ops-notify-payload.json"
    writeFile file: payloadFile, text: payload, encoding: 'UTF-8'
    withEnv(["OPS_NOTIFY_PAYLOAD_FILE=${payloadFile}"]) {
        withCredentials([string(credentialsId: 'ops-notify-token', variable: 'OPS_NOTIFY_TOKEN')]) {
            sh '''
                set +x
                curl --fail --silent --show-error --connect-timeout 5 --max-time 10 \
                  -H 'Content-Type: application/json' \
                  -H "Authorization: Bearer ${OPS_NOTIFY_TOKEN}" \
                  -X POST \
                  --data-binary @"${OPS_NOTIFY_PAYLOAD_FILE}" \
                  "${OPS_NOTIFY_URL}/api/v1/notifications/jenkins" \
                  || echo 'WARN: ops notify failed'
            '''
        }
    }
}
```

JSON 通过临时文件发送，Token 从环境变量读取并关闭回显。HTTP 4xx/5xx 或网络错误只输出警告；工作区、写文件或凭据错误仍会使步骤失败。

## 字段与展示

- 完整字段见[接口文档](接口文档.md)。`commit` 可传提交摘要；`imageRefs` 有值时展示镜像，否则展示 `packageType`。
- `deploy` 省略时不展示，true/false 显示“是/否”；示例参数应为 boolean 或 `true/false` 字符串。
- 仅 `deploy=true` 且 `deployType` 非空、不为 `none` 时展示部署方式，如“是（ssh）”；`deploy=false` 始终显示“否”。
- 机器人 webhook 和 secret 仅保存在[网关配置](配置说明.md)，不写入 Jenkinsfile。
