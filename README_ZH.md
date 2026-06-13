# GLM 多协议仿真器

[English](README.md) | 中文

这是一个面向 GLM 的协议仿真器。它对外提供 GLM Chat Completions、OpenAI Chat Completions、OpenAI Responses、Anthropic Messages 和 Gemini Generate Content 等兼容 API，并在 GLM Chat Completions 之上尽可能高保真地仿真这些协议的请求与响应语义。

本地运行时通过命令行参数进行配置，容器部署时通过环境变量进行配置。

## 配置与使用

本地二进制运行时，服务通过命令行参数配置。
可以拉取项目后自行编译运行或直接从 Release 下载：

```bash
git clone https://github.com/Joey-Kot/glm-compatible.git
cd glm-compatible
go build -trimpath -ldflags="-s -w" -o glm-compatible ./cmd/server
```

```bash
./glm-compatible \
  --listen :8080 \
  --api-token sk-local-test \
  --glm-api-key sk-your-glm-key \
  --glm-base-url https://api.z.ai/api \
  --glm-model glm-5.1 \
  --glm-models glm-5.1 \
  --glm-http-timeout 120 \
  --verify-ssl=true \
  --debug-log-body=false
```

容器部署时使用环境变量。可以先从 `docker.env.example` 复制一份配置：

```bash
cp docker.env.example docker.env
```

编辑 `docker.env` 后，可以直接拉取线上镜像部署：

```bash
docker run -itd \
  --name glm-compatible \
  -p 8080:8080 \
  --env-file docker.env \
  --restart always \
  ghcr.io/joey-kot/glm-compatible:latest
```

也可以拉取项目后自行构建镜像部署：

```bash
git clone https://github.com/Joey-Kot/glm-compatible.git
cd glm-compatible
docker build -t glm-compatible:latest .
```

```bash
docker run -itd \
  --name glm-compatible \
  -p 8080:8080 \
  --env-file docker.env \
  --restart always \
  glm-compatible:latest
```

容器环境变量说明：

| 环境变量 | 对应参数 |
| --- | --- |
| `LISTEN` | `--listen` |
| `API_TOKEN` | `--api-token` |
| `GLM_API_KEY` | `--glm-api-key` |
| `GLM_BASE_URL` | `--glm-base-url` |
| `GLM_MODEL` | `--glm-model` |
| `GLM_MODELS` | `--glm-models` |
| `GLM_HTTP_TIMEOUT` | `--glm-http-timeout` |
| `GLM_MAX_IDLE_CONNS` | `--glm-max-idle-conns` |
| `GLM_MAX_IDLE_CONNS_PER_HOST` | `--glm-max-idle-conns-per-host` |
| `GLM_MAX_CONNS_PER_HOST` | `--glm-max-conns-per-host` |
| `READ_HEADER_TIMEOUT` | `--read-header-timeout` |
| `IDLE_TIMEOUT` | `--idle-timeout` |
| `VERIFY_SSL` | `--verify-ssl` |
| `DEBUG_LOG_BODY` | `--debug-log-body` |

参数说明：

| 参数 | 描述 |
| --- | --- |
| `--listen` | 本地 HTTP 监听地址，默认 `:8080`。 |
| `--api-token` | 访问本兼容后端所需的本地 token，支持用逗号配置多个 token；OpenAI 风格请求使用 `Authorization: Bearer`，Anthropic 风格请求使用 `x-api-key`，Gemini 风格请求使用 `x-goog-api-key`。 |
| `--glm-api-key` | GLM 上游 API key。 |
| `--glm-base-url` | GLM 上游 base URL；不填写或填写为空字符串时使用默认值 `https://api.z.ai/api`，也可以填写 `http://` 或 `https://`，并且可以直接指向 `/paas/v4/chat/completions`。 |
| `--glm-model` | 默认转发到 GLM 的模型 ID，默认 `glm-5.1`。 |
| `--glm-models` | `/v1/models` 对外暴露的模型 ID 列表，多个模型用逗号分隔；如果未包含默认模型，会自动把默认模型放到列表前面。 |
| `--glm-http-timeout` | GLM 上游 HTTP 请求超时时间，单位为秒，默认 `120`。 |
| `--glm-max-idle-conns` | 上游 HTTP 空闲连接复用池总上限，默认 `200`。 |
| `--glm-max-idle-conns-per-host` | 每个上游 host 保留的空闲连接上限，默认 `100`。 |
| `--glm-max-conns-per-host` | 每个上游 host 的并发连接上限，默认 `0` 表示不限制。 |
| `--read-header-timeout` | 本地 HTTP 读取请求头超时时间，单位为秒，默认 `10`。 |
| `--idle-timeout` | 本地 HTTP 空闲连接超时时间，单位为秒，默认 `120`。 |
| `--verify-ssl` | 是否校验 GLM 上游 HTTPS 证书，默认 `true`；只有在可信代理或临时证书异常场景下才建议设为 `false`。 |
| `--debug-log-body` | 是否输出经过脱敏的本地请求/响应 body 和 GLM 上游请求/响应 body，默认 `false`；API key、token、password、secret 等字段会被替换为 `[REDACTED]`，日志长度也会被限制。 |

完整参数可以参考 `args.example`。

## 兼容端点

### GLM Chat Completions

| 端点 | 描述 |
| --- | --- |
| `POST /paas/v4/chat/completions` | 创建 GLM Chat Completions，请求和响应参数均与 GLM 官方 API 一一对应。 |

### OpenAI Chat Completions

| 端点 | 描述 |
| --- | --- |
| `POST /v1/chat/completions` | 创建 Chat Completions，并转发到 GLM Chat Completions。 |
| `GET /v1/chat/completions` | 列出本地保存的 Chat Completions。 |
| `GET /v1/chat/completions/{completion_id}` | 读取本地保存的单个 Chat Completion。 |
| `POST /v1/chat/completions/{completion_id}` | 更新本地保存的 Chat Completion 元数据。 |
| `DELETE /v1/chat/completions/{completion_id}` | 删除本地保存的 Chat Completion。 |
| `GET /v1/chat/completions/{completion_id}/messages` | 列出本地保存的 Chat Completion 消息。 |

### OpenAI Responses

| 端点 | 描述 |
| --- | --- |
| `POST /v1/responses` | 创建 Responses，并转发到 GLM Chat Completions。 |
| `GET /v1/responses/{response_id}` | 读取本地保存的单个 Response。 |
| `DELETE /v1/responses/{response_id}` | 删除本地保存的 Response。 |
| `GET /v1/responses/{response_id}/input_items` | 列出本地保存的 Response 输入项。 |
| `POST /v1/responses/{response_id}/cancel` | 按本地状态语义取消 Response。 |
| `POST /v1/responses/input_tokens` | 使用 GLM 上游 tokenizer API 统计输入 token。 |
| `POST /v1/responses/compact` | 使用 GLM 进行尽力而为的上下文压缩总结。 |

NOTICE：针对 Codex CLI 的 MCP namespace 工具调用，Responses 适配器会在转发给 GLM 前将 namespace 和工具名展开成 GLM 可接受的 function 名称，并在返回时尽量还原为 Responses 工具调用结构。

### OpenAI Conversations

| 端点 | 描述 |
| --- | --- |
| `POST /v1/conversations` | 创建本地 Conversation。 |
| `GET /v1/conversations/{conversation_id}` | 读取本地 Conversation。 |
| `POST /v1/conversations/{conversation_id}` | 追加或更新本地 Conversation。 |
| `DELETE /v1/conversations/{conversation_id}` | 删除本地 Conversation。 |

### Anthropic Messages

| 端点 | 描述 |
| --- | --- |
| `POST /v1/messages` | 创建 Anthropic Messages 响应，并转发到 GLM Chat Completions。 |
| `POST /v1/messages/count_tokens` | 使用 GLM 上游 tokenizer API 统计 Anthropic Messages token。 |

### Gemini Generate Content

| 端点 | 描述 |
| --- | --- |
| `POST /v1beta/models/{model}:generateContent` | 创建 Gemini Generate Content 响应，并转发到 GLM Chat Completions。 |
| `POST /v1beta/models/{model}:streamGenerateContent` | 创建 Gemini 流式 Generate Content 响应。 |
| `POST /v1beta/models/{model}:countTokens` | 使用 GLM 上游 tokenizer API 统计 Gemini v1beta token。 |
| `POST /v1/models/{model}:generateContent` | 创建 Gemini v1 Generate Content 响应，并转发到 GLM Chat Completions。 |
| `POST /v1/models/{model}:streamGenerateContent` | 创建 Gemini v1 流式 Generate Content 响应。 |
| `POST /v1/models/{model}:countTokens` | 使用 GLM 上游 tokenizer API 统计 Gemini v1 token。 |

### 通用端点

| 端点 | 描述 |
| --- | --- |
| `GET /v1/models` | 返回当前暴露给兼容客户端的模型列表。 |
| `GET /health` | 健康检查端点。 |

## 参数映射

### GLM Chat Completions

| GLM Chat Completions | GLM Chat Completions |
| --- | --- |
| 所有请求参数 | 原样转发到 GLM 上游。 |
| 非流式响应 | 原样返回 GLM 上游响应。 |
| 流式响应 | 原样转发 GLM 上游 SSE chunk。 |

`POST /paas/v4/chat/completions` 只进行本地鉴权、调试日志脱敏和上游错误处理，不做参数转换。

### OpenAI Chat Completions

| OpenAI Chat Completions | GLM Chat Completions |
| --- | --- |
| `model` | `model` |
| `messages` | `messages` |
| `developer` role | `system` role |
| `max_completion_tokens` / `max_tokens` | `max_tokens` |
| `temperature` | `temperature` |
| `top_p` | `top_p` |
| `stop` | `stop` |
| `presence_penalty` | `presence_penalty` |
| `frequency_penalty` | `frequency_penalty` |
| `logprobs` | `logprobs` |
| `top_logprobs` | `top_logprobs` |
| `n` | `n` |
| `seed` | `seed` |
| `stream` | GLM 流式响应转为 Chat Completions SSE chunk。 |
| `stream_options.include_usage` | `stream_options.include_usage` |
| `tools` function tools | `tools` |
| 已废弃的 `functions` | `tools` |
| `tool_choice` / 已废弃的 `function_call` | `tool_choice` |
| `tool_choice.type=allowed_tools` | 尽力过滤可用工具，并映射为 `auto` / `required`。 |
| `response_format.type=json_object` | `response_format={"type":"json_object"}` |
| `response_format.type=json_schema` | 尽力开启 JSON mode，并将 schema 写入提示。 |
| `reasoning_effort` | 尽力映射为 `thinking.type`；`none` / `minimal` 关闭 thinking，其余 reasoning 值开启 thinking。不会向 GLM 上游发送 `reasoning_effort`。 |
| `thinking` | GLM `thinking` |

OpenAI Chat Completions 官方响应结构不包含 reasoning summary；因此 GLM 返回的 `reasoning_content` 会作为 GLM 扩展字段保留并透传，不转换成 OpenAI Responses 的 `reasoning.summary[].summary_text` 结构。

当请求设置 `store=true` 时，Chat Completions 会保存在本地内存中。读取、更新 metadata、删除、按 `metadata[key]` 列表过滤，以及消息列表都属于本地兼容状态；GLM 本身不保存这些对象，服务重启后本地状态会丢失。

### OpenAI Responses

| OpenAI Responses | GLM Chat Completions |
| --- | --- |
| `model` | `model` |
| `input` string | `messages: [{role: "user", content: input}]` |
| `input` message items | `messages` |
| `instructions` | 前置 `system` message |
| `max_output_tokens` | `max_tokens` |
| `temperature` | `temperature` |
| `top_p` | `top_p` |
| `stop` | `stop` |
| `tools` function tools | Chat Completions `tools` |
| `tools` namespace function/custom tools，包括 Codex CLI MCP tools | 展平为 Chat Completions function tools |
| `tool_choice` for function/custom tools | Chat Completions `tool_choice` |
| `text.format.type=json_object` | `response_format={"type":"json_object"}` |
| `text.format.type=json_schema` | 尽力开启 JSON mode，并将 schema 写入提示。 |
| `reasoning.effort` | 尽力映射为 GLM `thinking.type`；不会向 GLM 上游发送 `reasoning_effort`。 |
| `stream=true` | GLM 流式响应转为 Responses SSE events。 |

GLM 返回的 `reasoning_content` 会映射为 Responses 输出中的 `reasoning.summary[].summary_text`；流式响应中会通过 reasoning summary 相关 SSE 事件逐段输出。Responses namespace tools，包括 Codex CLI 本地 MCP namespace 的工具形态，会在发送到 GLM 前展平为 function tools，并在返回时尽量还原为 `namespace` / `name` 结构。OpenAI 托管 web search 工具会尽量映射为 GLM `web_search` 工具。没有 GLM Chat Completions 近似语义的托管工具，例如 file search、code interpreter、image generation、computer use、remote MCP connectors、moderation 和后台队列，会被忽略。

### Anthropic Messages

| Anthropic Messages | GLM Chat Completions |
| --- | --- |
| `model` | `model` |
| `messages[].role=user/assistant` | `messages[].role=user/assistant` |
| 顶层 `system` | 前置 `system` message |
| text content blocks | message `content` text |
| `thinking` content blocks | assistant `reasoning_content` |
| `tool_use` blocks | assistant `tool_calls` |
| `tool_result` blocks | `tool` messages |
| `max_tokens` | `max_tokens` |
| `temperature` | `temperature` |
| `top_p` | `top_p` |
| `stop_sequences` | `stop` |
| `tools[].input_schema` | function tool `parameters` |
| `tool_choice.type=auto/any/none/tool` | `auto` / `required` / `none` / named function |
| `thinking.type=enabled/disabled` | GLM `thinking` |
| `output_config.format=json_schema` | 尽力开启 JSON mode，并将 schema 写入提示。 |
| `stream=true` | Anthropic Messages SSE events |

Anthropic 的 image、document、search-result 和 server-tool blocks 在所选 GLM 多模态模型支持对应原生输入时会尽量转换为原生多模态内容；否则会尽量转换为文本描述或省略。GLM 返回的 tool calls 会转换为 Anthropic `tool_use` content blocks。token counting 使用 GLM 上游 tokenizer API 完成。

### Gemini Generate Content

| Gemini Generate Content | GLM Chat Completions |
| --- | --- |
| path `{model}` | `model` |
| `contents[].role=user/model` | `user` / `assistant` messages |
| `contents[].parts[].text` | message `content` text |
| `systemInstruction.parts[].text` | 前置 `system` message |
| `functionCall` parts | assistant `tool_calls` |
| `functionResponse` parts | `tool` messages |
| `generationConfig.maxOutputTokens` | `max_tokens` |
| `generationConfig.temperature` | `temperature` |
| `generationConfig.topP` | `top_p` |
| `generationConfig.stopSequences` | `stop` |
| `generationConfig.responseMimeType=application/json` | `response_format={"type":"json_object"}` |
| `generationConfig.responseSchema` | 尽力开启 JSON mode，并将 schema 写入提示。 |
| `generationConfig.thinkingConfig` | 尽力映射为 GLM `thinking.type`；不会向 GLM 上游发送 `reasoning_effort`。 |
| `tools[].functionDeclarations` | function tools |
| `toolConfig.functionCallingConfig` | `tool_choice`，并尽力过滤可用工具。 |
| `:streamGenerateContent` | Gemini SSE chunks |

Gemini 多模态输入 parts 和内置工具会在所选 GLM 模型支持该模态时尽量转换为 GLM 原生多模态内容；否则会尽量转换为文本描述或省略。通过 `functionDeclarations` 定义的函数会映射为 GLM function tools；GLM 返回的 tool calls 会转换为 Gemini `functionCall` parts。

## 请求示例

健康检查：

```bash
curl http://localhost:8080/health
```

创建 GLM Chat Completion：

```bash
curl http://localhost:8080/paas/v4/chat/completions \
  -H "Authorization: Bearer sk-local-test" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "glm-5.1",
    "messages": [
      {"role": "system", "content": "You are a helpful assistant."},
      {"role": "user", "content": "Hello!"}
    ],
    "thinking": {"type": "enabled"}
  }'
```

创建 OpenAI Responses：

```bash
curl http://localhost:8080/v1/responses \
  -H "Authorization: Bearer sk-local-test" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "glm-5.1",
    "instructions": "You are a helpful assistant.",
    "input": "Hello!",
    "reasoning": {"effort": "high"}
  }'
```

创建 OpenAI Chat Completion：

```bash
curl http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer sk-local-test" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "glm-5.1",
    "messages": [
      {"role": "system", "content": "You are a helpful assistant."},
      {"role": "user", "content": "Hello!"}
    ],
    "reasoning_effort": "high"
  }'
```

创建 Anthropic Message：

```bash
curl http://localhost:8080/v1/messages \
  -H "x-api-key: sk-local-test" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "glm-5.1",
    "max_tokens": 128,
    "system": "You are a helpful assistant.",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'
```

创建 Gemini Generate Content：

```bash
curl http://localhost:8080/v1beta/models/gemini-3.5-flash:generateContent \
  -H "x-goog-api-key: sk-local-test" \
  -H "Content-Type: application/json" \
  -d '{
    "contents": [{
      "role": "user",
      "parts": [{"text": "Hello!"}]
    }]
  }'
```

创建 OpenAI Responses 流式响应：

```bash
curl http://localhost:8080/v1/responses \
  -H "Authorization: Bearer sk-local-test" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "glm-5.1",
    "input": "Write one haiku.",
    "stream": true
  }'
```

使用 OpenAI Responses function tool：

```bash
curl http://localhost:8080/v1/responses \
  -H "Authorization: Bearer sk-local-test" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "glm-5.1",
    "input": "What is the weather in New York?",
    "tools": [{
      "type": "function",
      "name": "get_weather",
      "description": "Get weather by city.",
      "parameters": {
        "type": "object",
        "properties": {"city": {"type": "string"}},
        "required": ["city"]
      }
    }],
    "tool_choice": "auto"
  }'
```

## 兼容性说明

本后端会在内存中保存 Responses 和 Conversations 状态，用于支持 `previous_response_id`、`conversation`、读取、删除和输入项列表等兼容能力。如果没有接入外部存储，服务重启后这些本地状态会丢失。

Token counting 端点会调用 GLM 上游 tokenizer API（`/paas/v4/tokenizer`），再将结果包装成目标协议的响应结构。

`POST /v1/responses/{id}/cancel` 只能按本地状态语义标记尚未完成的 Response。普通请求是同步完成的，GLM Chat Completions 也不提供 OpenAI 后台任务式执行能力。

GLM 可能返回 `reasoning_content`。本后端会在目标 API 存在结构化 reasoning 形态时进行映射：OpenAI Responses 映射为 `reasoning.summary[].summary_text`，Anthropic 映射为 `thinking` blocks，Gemini 映射为 `thought` parts。OpenAI Chat Completions 官方响应结构不包含 reasoning summary，因此 Chat Completions 会保留并透传 GLM 的 `reasoning_content` 扩展字段。

## 许可证

本项目基于 GNU General Public License v3.0 or later（GPLv3+）授权。详情请查看 [LICENSE](LICENSE)。
