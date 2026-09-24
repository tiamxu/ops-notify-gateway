# Jenkins 接入说明

Jenkins 不需要直接保存钉钉或飞书 webhook，只调用 `ops-notify-gateway`。

## Pipeline 示例

```groovy
post {
    success {
        script {
            notifyBuild('SUCCESS')
        }
    }
    failure {
        script {
            notifyBuild('FAILURE')
        }
    }
    aborted {
        script {
            notifyBuild('ABORTED')
        }
    }
    unstable {
        script {
            notifyBuild('UNSTABLE')
        }
    }
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
        packageType: params.PACKAGE_TYPE ?: '',
        deploy: params.DEPLOY ?: false,
        deployType: params.DEPLOY_TYPE ?: '',
        triggerUser: currentBuild.getBuildCauses('hudson.model.Cause$UserIdCause')?.getAt(0)?.userId ?: 'unknown'
    ])

    withCredentials([string(credentialsId: 'ops-notify-token', variable: 'OPS_NOTIFY_TOKEN')]) {
        sh """
            curl -sS --connect-timeout 5 --max-time 10 \
              -H 'Content-Type: application/json' \
              -H "Authorization: Bearer ${OPS_NOTIFY_TOKEN}" \
              -X POST \
              --data '${payload}' \
              '${env.OPS_NOTIFY_URL}/api/v1/notifications/jenkins' \
              || echo 'WARN: ops notify failed'
        """
    }
}
```

## Jenkins 环境变量建议

```text
OPS_NOTIFY_URL=http://ops-notify-gateway:8801
OPS_NOTIFY_CHANNEL=jenkins-test
```

`OPS_NOTIFY_TOKEN` 建议放 Jenkins Credentials，类型为 Secret text。

## 注意事项

- 通知失败不应影响构建结果。
- 不要在 Jenkinsfile 中写钉钉、飞书 webhook。
- 不要在 Jenkinsfile 中写加签 secret。