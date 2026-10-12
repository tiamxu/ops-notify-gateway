#!/bin/sh
set -eu

BASE_URL="${BASE_URL:-http://127.0.0.1:8801}"
TOKEN="${TOKEN:-change-me}"
TEST_CASE="${TEST_CASE:-basic}"
GENERIC_CHANNEL="${GENERIC_CHANNEL:-ops-test}"
JENKINS_CHANNEL="${JENKINS_CHANNEL:-jenkins-test}"
ALERT_CHANNEL="${ALERT_CHANNEL:-alert-prod}"

request() {
  name="$1"
  shift
  echo "==> ${name}"
  curl --fail --show-error --silent "$@"
  echo
  echo
}

request_allow_failure() {
  name="$1"
  shift
  echo "==> ${name}"
  curl --show-error --silent "$@" || true
  echo
  echo
}

request_with_status() {
  name="$1"
  shift
  tmp="${TMPDIR:-/tmp}/ops_notify_response.$$"
  echo "==> ${name}"
  status=$(curl --show-error --silent --output "${tmp}" --write-out "%{http_code}" "$@") || {
    code="$?"
    if [ -f "${tmp}" ]; then
      cat "${tmp}"
      rm -f "${tmp}"
    fi
    echo
    return "${code}"
  }
  cat "${tmp}"
  rm -f "${tmp}"
  echo
  echo "HTTP_STATUS=${status}"
  echo
  case "${status}" in
    2*) return 0 ;;
    *) return 1 ;;
  esac
}

run_basic_tests() {
  request "ping" \
    "${BASE_URL}/ping"

  request_allow_failure "auth rejected" \
    --request POST \
    --header "Content-Type: application/json" \
    --data '{}' \
    "${BASE_URL}/api/v1/notifications/generic"

  request_allow_failure "jenkins invalid json" \
    --request POST \
    --header "Content-Type: application/json" \
    --header "Authorization: Bearer ${TOKEN}" \
    --data '{' \
    "${BASE_URL}/api/v1/notifications/jenkins"

  request_allow_failure "generic invalid json" \
    --request POST \
    --header "Content-Type: application/json" \
    --header "Authorization: Bearer ${TOKEN}" \
    --data '{' \
    "${BASE_URL}/api/v1/notifications/generic"
}

run_generic_test() {
  request_with_status "generic notification" \
    --request POST \
    --header "Content-Type: application/json" \
    --header "Authorization: Bearer ${TOKEN}" \
    --data "{\"channel\":\"${GENERIC_CHANNEL}\",\"title\":\"curl 测试通知\",\"status\":\"SUCCESS\",\"level\":\"info\",\"content\":\"Linux curl 测试内容\",\"url\":\"https://example.com\"}" \
    "${BASE_URL}/api/v1/notifications/generic"
}

run_jenkins_test() {
  request_with_status "jenkins notification" \
    --request POST \
    --header "Content-Type: application/json" \
    --header "Authorization: Bearer ${TOKEN}" \
    --data "{\"channel\":\"${JENKINS_CHANNEL}\",\"status\":\"SUCCESS\",\"jobName\":\"demo-job\",\"buildNumber\":\"12\",\"buildUrl\":\"https://jenkins/job/demo/12/\",\"services\":[\"demo-service\"],\"branch\":\"main\",\"commit\":\"a1b2c3d admin update service\",\"deployEnv\":\"test\",\"imageRefs\":[\"registry-test.example.com/demo/demo-service:test-12\"],\"packageType\":\"image\",\"deploy\":true,\"deployType\":\"ssh\",\"triggerUser\":\"admin\"}" \
    "${BASE_URL}/api/v1/notifications/jenkins"
}

run_alertmanager_test() {
  request_with_status "alertmanager notification" \
    --request POST \
    --header "Content-Type: application/json" \
    --header "Authorization: Bearer ${TOKEN}" \
    --data "{\"status\":\"firing\",\"groupLabels\":{\"alertname\":\"DemoAlert\"},\"commonLabels\":{\"severity\":\"warning\"},\"alerts\":[{\"status\":\"firing\",\"labels\":{\"instance\":\"127.0.0.1\"},\"annotations\":{\"summary\":\"测试告警\",\"description\":\"curl 触发测试\"},\"startsAt\":\"2026-10-09T10:00:00Z\"}]}" \
    "${BASE_URL}/api/v1/alerts/alertmanager?channel=${ALERT_CHANNEL}&split=false"
}

case "${TEST_CASE}" in
  basic)
    run_basic_tests
    echo "Skip notification send tests. Set TEST_CASE=generic, jenkins, alertmanager, or all to send real notifications."
    ;;
  generic)
    run_generic_test
    ;;
  jenkins)
    run_jenkins_test
    ;;
  alertmanager)
    run_alertmanager_test
    ;;
  all)
    run_basic_tests
    run_generic_test
    run_jenkins_test
    run_alertmanager_test
    ;;
  *)
    echo "Unsupported TEST_CASE: ${TEST_CASE}" >&2
    echo "Allowed values: basic, generic, jenkins, alertmanager, all" >&2
    exit 2
    ;;
esac