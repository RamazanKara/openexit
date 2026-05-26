# OpenAI/Anthropic to vLLM/LiteLLM

This assessment path evaluates AI provider usage and generates conservative migration artifacts for a self-hosted vLLM runtime behind LiteLLM routing. It supports local OpenAI/Anthropic-style fixtures and a read-only live OpenAI aggregate usage collector. Anthropic is currently fixture-only.

## Fixture Scope

The fixture collector captures:

- Model usage classes and source model names.
- Monthly and peak token volume profiles.
- Latency expectations, including streaming requirements.
- Sensitive prompt categories and retention metadata.
- Tool usage, including network and write behavior.
- Fallback behavior for outage or degraded-mode handling.

OpenExit does not collect provider credentials or raw prompts.

## Live OpenAI Scope

The live OpenAI collector captures:

- Available model IDs from the OpenAI models API.
- Model-grouped aggregate completions usage over a configurable window.
- Input and output token totals, including audio token fields when present.
- Hourly peak token estimates over a configurable recent window.
- Optional owner, latency, streaming, and fallback metadata supplied as CLI flags.

The OpenAI usage API does not expose application-level prompt sensitivity, tool behavior, latency objectives, or fallback design. When those are not supplied as flags, OpenExit leaves them absent so the assessment can raise explicit manual-review findings.

The collector writes redacted evidence only. It does not collect raw prompts, completions, provider secrets, API key values, or request/response content.

API surface used:

- OpenAI organization completions usage API: `GET /v1/organization/usage/completions`.
- OpenAI models API: `GET /v1/models`.

## Demo

```bash
openexit init ./ai-demo --source ai-provider --target vllm-litellm
openexit collect ai-fixture --project ./ai-demo --input ./testdata/ai-provider/small.json
openexit assess --project ./ai-demo --target vllm-litellm
openexit generate --project ./ai-demo --all
openexit validate --project ./ai-demo
```

## Live OpenAI Demo

```bash
export OPENAI_ADMIN_KEY=<admin-key>
openexit init ./openai-live --source ai-provider --target vllm-litellm
openexit collect openai \
  --project ./openai-live \
  --admin-key-env OPENAI_ADMIN_KEY \
  --workspace acme \
  --owner platform-ai \
  --fallback-strategy manual-queue \
  --fallback-manual-queue
openexit assess --project ./openai-live --target vllm-litellm
openexit generate --project ./openai-live --all
openexit validate --project ./openai-live
```

## Generated Artifacts

- `assessment/self-hosted-llm-readiness-report.md`
- `assessment/vllm-sizing-assumptions.md`
- `assessment/evaluation-plan.md`
- `assessment/data-sensitivity-report.md`
- `generated-config/ai/litellm/config.candidate.yaml`

All generated target files are candidates and require human review, model evaluation, load testing, data-control review, and tool-policy review before production use.
