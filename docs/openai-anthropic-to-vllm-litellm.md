# OpenAI/Anthropic to vLLM/LiteLLM Preview

This preview assesses AI provider usage from local fixture data and generates conservative migration artifacts for a self-hosted vLLM runtime behind LiteLLM routing.

## Fixture Scope

The fixture collector captures:

- Model usage classes and source model names.
- Monthly and peak token volume profiles.
- Latency expectations, including streaming requirements.
- Sensitive prompt categories and retention metadata.
- Tool usage, including network and write behavior.
- Fallback behavior for outage or degraded-mode handling.

OpenExit does not collect provider credentials or raw prompts.

## Demo

```bash
openexit init ./ai-demo
openexit collect ai-fixture --project ./ai-demo --input ./testdata/ai-provider/small.json
openexit assess --project ./ai-demo --target vllm-litellm
openexit generate --project ./ai-demo --all
openexit validate --project ./ai-demo
```

## Generated Artifacts

- `assessment/self-hosted-llm-readiness-report.md`
- `assessment/vllm-sizing-assumptions.md`
- `assessment/evaluation-plan.md`
- `assessment/data-sensitivity-report.md`
- `generated-config/ai/litellm/config.candidate.yaml`

All generated target files are candidates and require human review, model evaluation, load testing, data-control review, and tool-policy review before production use.
