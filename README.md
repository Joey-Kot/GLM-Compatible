# GLM Multi-Protocol Emulator

English | [中文](README_ZH.md)

This is a GLM-focused protocol emulator. It exposes GLM Chat Completions, OpenAI Chat Completions, OpenAI Responses, Anthropic Messages, and Gemini Generate Content compatible APIs, then emulates their request and response semantics on top of GLM Chat Completions as faithfully as possible.

It is configured with command-line flags for local runs and environment variables for container deployment.

## Configuration and Usage

For local binary runs, configure the service with command-line flags. You can clone the project and build it locally, or download a prebuilt binary from the Release page:

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

For container deployment, use environment variables. You can start from `docker.env.example`:

```bash
cp docker.env.example docker.env
```

Then edit `docker.env` and deploy directly with the published container image:

```bash
docker run -itd \
  --name glm-compatible \
  -p 8080:8080 \
  --env-file docker.env \
  --restart always \
  ghcr.io/joey-kot/glm-compatible:latest
```

Or clone the project and build the container image yourself:

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

Container environment reference:

| Environment variable | Equivalent flag |
| --- | --- |
| `LISTEN` | `--listen` |
| `API_TOKEN` | `--api-token` |
| `GLM_API_KEY` | `--glm-api-key` |
| `GLM_BASE_URL` | `--glm-base-url` |
| `GLM_MODEL` | `--glm-model` |
| `GLM_MODELS` | `--glm-models` |
| `STORE_TTL` | `--store-ttl` |
| `STORE_MAX_RESPONSES` | `--store-max-responses` |
| `STORE_MAX_CHAT_COMPLETIONS` | `--store-max-chat-completions` |
| `STORE_MAX_CONVERSATIONS` | `--store-max-conversations` |
| `STORE_PRUNE_INTERVAL` | `--store-prune-interval` |
| `GLM_HTTP_TIMEOUT` | `--glm-http-timeout` |
| `GLM_MAX_RESPONSE_BODY_BYTES` | `--glm-max-response-body-bytes` |
| `GLM_MAX_IDLE_CONNS` | `--glm-max-idle-conns` |
| `GLM_MAX_IDLE_CONNS_PER_HOST` | `--glm-max-idle-conns-per-host` |
| `GLM_MAX_CONNS_PER_HOST` | `--glm-max-conns-per-host` |
| `MAX_REQUEST_BODY_BYTES` | `--max-request-body-bytes` |
| `READ_HEADER_TIMEOUT` | `--read-header-timeout` |
| `IDLE_TIMEOUT` | `--idle-timeout` |
| `VERIFY_SSL` | `--verify-ssl` |
| `DEBUG_LOG_BODY` | `--debug-log-body` |
| `DEBUG_PPROF` | `--debug-pprof` |

Flag reference:

| Flag | Description |
| --- | --- |
| `--listen` | Local HTTP listen address. Defaults to `:8080`. |
| `--api-token` | Local token required to access this compatibility backend. Multiple tokens can be configured with commas. OpenAI-style requests use `Authorization: Bearer`, Anthropic-style requests use `x-api-key`, and Gemini-style requests use `x-goog-api-key`. |
| `--glm-api-key` | GLM upstream API key. |
| `--glm-base-url` | GLM upstream base URL. If omitted or set to an empty string, the default `https://api.z.ai/api` is used. It may be `http://` or `https://`, and it may point directly to `/paas/v4/chat/completions`. |
| `--glm-model` | Default model ID forwarded to GLM. Defaults to `glm-5.1`. |
| `--glm-models` | Model IDs exposed by `/v1/models`, separated by commas. If the default model is not included, it is automatically inserted at the front of the list. |
| `--store-ttl` | Local in-memory store TTL in seconds. Defaults to `3600`; set to `0` to disable TTL. |
| `--store-max-responses` | Maximum locally stored Responses entries. Defaults to `1000`; set to `0` to disable this limit. |
| `--store-max-chat-completions` | Maximum locally stored Chat Completions entries. Defaults to `1000`; set to `0` to disable this limit. |
| `--store-max-conversations` | Maximum locally stored Conversations entries. Defaults to `1000`; set to `0` to disable this limit. |
| `--store-prune-interval` | Minimum seconds between in-memory store prune checks on request paths. Defaults to `60`. |
| `--glm-http-timeout` | GLM upstream HTTP timeout in seconds. Defaults to `120`. |
| `--glm-max-response-body-bytes` | Maximum upstream GLM non-streaming or error response body size in bytes. Defaults to `33554432`; set to `0` to disable this limit. |
| `--glm-max-idle-conns` | Maximum idle upstream HTTP connections kept for reuse. Defaults to `200`. |
| `--glm-max-idle-conns-per-host` | Maximum idle upstream HTTP connections kept per host. Defaults to `100`. |
| `--glm-max-conns-per-host` | Maximum concurrent upstream HTTP connections per host. Defaults to `0`, which means unlimited. |
| `--max-request-body-bytes` | Maximum local request body size in bytes. Defaults to `16777216`; set to `0` to disable this limit. |
| `--read-header-timeout` | Local HTTP read header timeout in seconds. Defaults to `10`. |
| `--idle-timeout` | Local HTTP idle connection timeout in seconds. Defaults to `120`. |
| `--verify-ssl` | Whether to verify the GLM upstream HTTPS certificate. Defaults to `true`; set to `false` only for trusted proxies or temporary certificate problems. |
| `--debug-log-body` | Whether to log redacted local request/response bodies and GLM upstream request/response bodies. Defaults to `false`; API keys, tokens, passwords, secrets, and similar fields are replaced with `[REDACTED]`, and log length is capped. |
| `--debug-pprof` | Whether to enable authenticated `/debug/pprof/` and `/debug/vars` endpoints. Defaults to `false`. |

See `args.example` for the full flag set.

## Compatible Endpoints

### GLM Chat Completions

| Endpoint | Description |
| --- | --- |
| `POST /paas/v4/chat/completions` | Create GLM Chat Completions. Request and response parameters correspond one-to-one with the official GLM API. |

### OpenAI Chat Completions

| Endpoint | Description |
| --- | --- |
| `POST /v1/chat/completions` | Create Chat Completions and forward them to GLM Chat Completions. |
| `GET /v1/chat/completions` | List locally stored Chat Completions. |
| `GET /v1/chat/completions/{completion_id}` | Retrieve one locally stored Chat Completion. |
| `POST /v1/chat/completions/{completion_id}` | Update metadata for a locally stored Chat Completion. |
| `DELETE /v1/chat/completions/{completion_id}` | Delete a locally stored Chat Completion. |
| `GET /v1/chat/completions/{completion_id}/messages` | List messages for a locally stored Chat Completion. |

### OpenAI Responses

| Endpoint | Description |
| --- | --- |
| `POST /v1/responses` | Create Responses and forward them to GLM Chat Completions. |
| `GET /v1/responses/{response_id}` | Retrieve one locally stored Response. |
| `DELETE /v1/responses/{response_id}` | Delete a locally stored Response. |
| `GET /v1/responses/{response_id}/input_items` | List input items for a locally stored Response. |
| `POST /v1/responses/{response_id}/cancel` | Cancel a Response according to local state semantics. |
| `POST /v1/responses/input_tokens` | Count input tokens with the GLM upstream tokenizer API. |
| `POST /v1/responses/compact` | Use GLM for best-effort context compaction and summarization. |

NOTICE: For Codex CLI MCP namespace tool calls, the Responses adapter expands namespace and tool names into GLM-compatible function names before forwarding them to GLM, and then tries to restore the Responses tool call structure on the way back.

### OpenAI Conversations

| Endpoint | Description |
| --- | --- |
| `POST /v1/conversations` | Create a local Conversation. |
| `GET /v1/conversations/{conversation_id}` | Retrieve a local Conversation. |
| `POST /v1/conversations/{conversation_id}` | Append to or update a local Conversation. |
| `DELETE /v1/conversations/{conversation_id}` | Delete a local Conversation. |

### Anthropic Messages

| Endpoint | Description |
| --- | --- |
| `POST /v1/messages` | Create an Anthropic Messages response and forward it to GLM Chat Completions. |
| `POST /v1/messages/count_tokens` | Count Anthropic Messages tokens with the GLM upstream tokenizer API. |

### Gemini Generate Content

| Endpoint | Description |
| --- | --- |
| `POST /v1beta/models/{model}:generateContent` | Create a Gemini Generate Content response and forward it to GLM Chat Completions. |
| `POST /v1beta/models/{model}:streamGenerateContent` | Create a streaming Gemini Generate Content response. |
| `POST /v1beta/models/{model}:countTokens` | Count Gemini v1beta tokens with the GLM upstream tokenizer API. |
| `POST /v1/models/{model}:generateContent` | Create a Gemini v1 Generate Content response and forward it to GLM Chat Completions. |
| `POST /v1/models/{model}:streamGenerateContent` | Create a streaming Gemini v1 Generate Content response. |
| `POST /v1/models/{model}:countTokens` | Count Gemini v1 tokens with the GLM upstream tokenizer API. |

### Common Endpoints

| Endpoint | Description |
| --- | --- |
| `GET /v1/models` | Return the model list exposed to compatible clients. |
| `GET /health` | Health check endpoint. |
| `GET /healthz/memory` | Authenticated memory and local store statistics endpoint. |

## Parameter Mapping

### GLM Chat Completions

| GLM Chat Completions | GLM Chat Completions |
| --- | --- |
| All request parameters | Forwarded to the GLM upstream unchanged. |
| Non-streaming response | Returned from the GLM upstream unchanged. |
| Streaming response | GLM upstream SSE chunks are forwarded unchanged. |

`POST /paas/v4/chat/completions` only performs local authentication, redacted debug logging, and upstream error handling. It does not transform parameters.

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
| `stream` | GLM streaming responses are converted to Chat Completions SSE chunks. |
| `stream_options.include_usage` | `stream_options.include_usage` |
| `tools` function tools | `tools` |
| Deprecated `functions` | `tools` |
| `tool_choice` / deprecated `function_call` | `tool_choice` |
| `tool_choice.type=allowed_tools` | Best-effort filtering of available tools, mapped to `auto` / `required`. |
| `response_format.type=json_object` | `response_format={"type":"json_object"}` |
| `response_format.type=json_schema` | JSON mode is enabled on a best-effort basis, and the schema is written into the prompt. |
| `reasoning_effort` | Best-effort mapping to `thinking.type`; `none` / `minimal` disable thinking, other reasoning values enable it. GLM does not receive `reasoning_effort`. |
| `thinking` | GLM `thinking` |

The official OpenAI Chat Completions response structure does not include reasoning summary. Therefore, GLM `reasoning_content` is preserved and passed through as a GLM extension field, instead of being converted to the OpenAI Responses `reasoning.summary[].summary_text` structure.

When a request sets `store=true`, Chat Completions are stored in local memory. Retrieval, metadata updates, deletion, list filtering by `metadata[key]`, and message listing are local compatibility state. GLM itself does not store these objects, and local state is lost after service restart.

### OpenAI Responses

| OpenAI Responses | GLM Chat Completions |
| --- | --- |
| `model` | `model` |
| `input` string | `messages: [{role: "user", content: input}]` |
| `input` message items | `messages` |
| `instructions` | Prepended `system` message |
| `max_output_tokens` | `max_tokens` |
| `temperature` | `temperature` |
| `top_p` | `top_p` |
| `stop` | `stop` |
| `tools` function tools | Chat Completions `tools` |
| `tools` namespace function/custom tools, including Codex CLI MCP tools | Flattened to Chat Completions function tools |
| `tool_choice` for function/custom tools | Chat Completions `tool_choice` |
| `text.format.type=json_object` | `response_format={"type":"json_object"}` |
| `text.format.type=json_schema` | JSON mode is enabled on a best-effort basis, and the schema is written into the prompt. |
| `reasoning.effort` | Best-effort mapping to GLM `thinking.type`; GLM does not receive `reasoning_effort`. |
| `stream=true` | GLM streaming responses are converted to Responses SSE events. |

GLM `reasoning_content` is mapped to `reasoning.summary[].summary_text` in Responses output. In streaming responses, it is emitted incrementally through reasoning summary SSE events. Responses namespace tools, including the local MCP namespace tool shape used by Codex CLI, are flattened to function tools before being sent to GLM, and are restored to `namespace` / `name` structure on a best-effort basis. OpenAI hosted web search tools are mapped to GLM `web_search` tools when possible. Hosted tools without close GLM Chat Completions equivalents, such as file search, code interpreter, image generation, computer use, remote MCP connectors, moderation, and background queues, are ignored.

### Anthropic Messages

| Anthropic Messages | GLM Chat Completions |
| --- | --- |
| `model` | `model` |
| `messages[].role=user/assistant` | `messages[].role=user/assistant` |
| Top-level `system` | Prepended `system` message |
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
| `output_config.format=json_schema` | JSON mode is enabled on a best-effort basis, and the schema is written into the prompt. |
| `stream=true` | Anthropic Messages SSE events |

Anthropic image, document, search-result, and server-tool blocks are converted to text descriptions on a best-effort basis and sent as GLM context unless a selected GLM multimodal model supports an equivalent native input. Tool calls returned by GLM are converted to Anthropic `tool_use` content blocks. Token counting is performed with the GLM upstream tokenizer API.

### Gemini Generate Content

| Gemini Generate Content | GLM Chat Completions |
| --- | --- |
| path `{model}` | `model` |
| `contents[].role=user/model` | `user` / `assistant` messages |
| `contents[].parts[].text` | message `content` text |
| `systemInstruction.parts[].text` | Prepended `system` message |
| `functionCall` parts | assistant `tool_calls` |
| `functionResponse` parts | `tool` messages |
| `generationConfig.maxOutputTokens` | `max_tokens` |
| `generationConfig.temperature` | `temperature` |
| `generationConfig.topP` | `top_p` |
| `generationConfig.stopSequences` | `stop` |
| `generationConfig.responseMimeType=application/json` | `response_format={"type":"json_object"}` |
| `generationConfig.responseSchema` | JSON mode is enabled on a best-effort basis, and the schema is written into the prompt. |
| `generationConfig.thinkingConfig` | Best-effort mapping to GLM `thinking.type`; GLM does not receive `reasoning_effort`. |
| `tools[].functionDeclarations` | function tools |
| `toolConfig.functionCallingConfig` | `tool_choice`, with best-effort filtering of available tools. |
| `:streamGenerateContent` | Gemini SSE chunks |

Gemini multimodal input parts and built-in tools are converted to native GLM multimodal content only when the selected GLM model supports that modality; otherwise they are converted to text descriptions or omitted on a best-effort basis. Functions defined through `functionDeclarations` are mapped to GLM function tools. Tool calls returned by GLM are converted to Gemini `functionCall` parts.

## Request Examples

Health check:

```bash
curl http://localhost:8080/health
```

Create a GLM Chat Completion:

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

Create OpenAI Responses:

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

Create an OpenAI Chat Completion:

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

Create an Anthropic Message:

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

Create Gemini Generate Content:

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

Create a streaming OpenAI Responses response:

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

Use an OpenAI Responses function tool:

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

## Compatibility Notes

This backend stores Responses, Chat Completions, and Conversations state in memory to support compatibility features such as `previous_response_id`, `conversation`, retrieval, deletion, list filtering, metadata updates, and input item listing. Without external storage, this local state is lost after service restart.

### In-Memory Store Reclamation

By default, local `store=true` state is reclaimed on request paths and does not rely on a background goroutine:

| Flag | Default | Description |
| --- | --- | --- |
| `--store-ttl` | `3600` | Keep entries for this many seconds since last access; `0` disables TTL. |
| `--store-max-responses` | `1000` | Maximum Responses entries; `0` disables this limit. |
| `--store-max-chat-completions` | `1000` | Maximum Chat Completions entries; `0` disables this limit. |
| `--store-max-conversations` | `1000` | Maximum Conversations entries; `0` disables this limit. |
| `--store-prune-interval` | `60` | Minimum seconds between prune checks on request paths. |

When a capacity limit is exceeded, the oldest saved entry of that type is evicted. Evicting a Response or Conversation also removes `ItemsByID` entries that are no longer referenced by any remaining Response or Conversation. Request bodies are limited by `--max-request-body-bytes 16777216` by default; upstream non-streaming and error response bodies are limited by `--glm-max-response-body-bytes 33554432` by default. Set either limit to `0` to disable it.

Use the authenticated memory endpoint to observe memory and store counts:

```bash
curl -H "Authorization: Bearer sk-local-test" \
  http://127.0.0.1:8080/healthz/memory
```

For pprof, explicitly start the service with `--debug-pprof=true`, then access authenticated `/debug/pprof/` or `/debug/vars`. Debug endpoints are disabled by default.

Token counting endpoints call the GLM upstream tokenizer API (`/paas/v4/tokenizer`) and wrap the result in the target protocol's response shape.

`POST /v1/responses/{id}/cancel` can only mark not-yet-completed Responses according to local state semantics. Normal requests complete synchronously, and GLM Chat Completions does not provide OpenAI-style background task execution.

GLM may return `reasoning_content`. This backend maps it to the target API's structured reasoning shape when one exists: OpenAI Responses maps it to `reasoning.summary[].summary_text`, Anthropic maps it to `thinking` blocks, and Gemini maps it to `thought` parts. The official OpenAI Chat Completions response structure does not include reasoning summary, so Chat Completions preserves and passes through GLM `reasoning_content` as an extension field.

## License

This project is licensed under the GNU General Public License v3.0 or later (GPLv3+). See [LICENSE](LICENSE) for details.
