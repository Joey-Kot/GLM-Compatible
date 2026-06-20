#!/bin/sh
# Copyright (C) 2026 Joey Kot <joey.kot.x@gmail.com>
#
# This program is free software: you can redistribute it and/or modify
# it under the terms of the GNU General Public License as published by
# the Free Software Foundation, either version 3 of the License, or
# (at your option) any later version.

set -eu

if [ "$#" -eq 0 ] || [ "${1#-}" != "$1" ]; then
  if [ -n "${DEBUG_LOG_BODY:-}" ]; then
    set -- "--debug-log-body=${DEBUG_LOG_BODY}" "$@"
  fi
  if [ -n "${VERIFY_SSL:-}" ]; then
    set -- "--verify-ssl=${VERIFY_SSL}" "$@"
  fi
  if [ -n "${DEBUG_PPROF:-}" ]; then
    set -- "--debug-pprof=${DEBUG_PPROF}" "$@"
  fi
  if [ -n "${MAX_REQUEST_BODY_BYTES:-}" ]; then
    set -- "--max-request-body-bytes" "${MAX_REQUEST_BODY_BYTES}" "$@"
  fi
  if [ -n "${GLM_HTTP_TIMEOUT:-}" ]; then
    set -- "--glm-http-timeout" "${GLM_HTTP_TIMEOUT}" "$@"
  fi
  if [ -n "${GLM_MAX_RESPONSE_BODY_BYTES:-}" ]; then
    set -- "--glm-max-response-body-bytes" "${GLM_MAX_RESPONSE_BODY_BYTES}" "$@"
  fi
  if [ -n "${GLM_MAX_IDLE_CONNS:-}" ]; then
    set -- "--glm-max-idle-conns" "${GLM_MAX_IDLE_CONNS}" "$@"
  fi
  if [ -n "${GLM_MAX_IDLE_CONNS_PER_HOST:-}" ]; then
    set -- "--glm-max-idle-conns-per-host" "${GLM_MAX_IDLE_CONNS_PER_HOST}" "$@"
  fi
  if [ -n "${GLM_MAX_CONNS_PER_HOST:-}" ]; then
    set -- "--glm-max-conns-per-host" "${GLM_MAX_CONNS_PER_HOST}" "$@"
  fi
  if [ -n "${STORE_PRUNE_INTERVAL:-}" ]; then
    set -- "--store-prune-interval" "${STORE_PRUNE_INTERVAL}" "$@"
  fi
  if [ -n "${STORE_MAX_CONVERSATIONS:-}" ]; then
    set -- "--store-max-conversations" "${STORE_MAX_CONVERSATIONS}" "$@"
  fi
  if [ -n "${STORE_MAX_CHAT_COMPLETIONS:-}" ]; then
    set -- "--store-max-chat-completions" "${STORE_MAX_CHAT_COMPLETIONS}" "$@"
  fi
  if [ -n "${STORE_MAX_RESPONSES:-}" ]; then
    set -- "--store-max-responses" "${STORE_MAX_RESPONSES}" "$@"
  fi
  if [ -n "${STORE_TTL:-}" ]; then
    set -- "--store-ttl" "${STORE_TTL}" "$@"
  fi
  if [ -n "${READ_HEADER_TIMEOUT:-}" ]; then
    set -- "--read-header-timeout" "${READ_HEADER_TIMEOUT}" "$@"
  fi
  if [ -n "${IDLE_TIMEOUT:-}" ]; then
    set -- "--idle-timeout" "${IDLE_TIMEOUT}" "$@"
  fi
  if [ -n "${GLM_MODELS:-}" ]; then
    set -- "--glm-models" "${GLM_MODELS}" "$@"
  fi
  if [ -n "${GLM_MODEL:-}" ]; then
    set -- "--glm-model" "${GLM_MODEL}" "$@"
  fi
  if [ -n "${GLM_BASE_URL:-}" ]; then
    set -- "--glm-base-url" "${GLM_BASE_URL}" "$@"
  fi
  if [ -n "${GLM_API_KEY:-}" ]; then
    set -- "--glm-api-key" "${GLM_API_KEY}" "$@"
  fi
  if [ -n "${API_TOKEN:-}" ]; then
    set -- "--api-token" "${API_TOKEN}" "$@"
  fi
  if [ -n "${LISTEN:-}" ]; then
    set -- "--listen" "${LISTEN}" "$@"
  fi

  set -- glm-compatible "$@"
fi

exec "$@"
