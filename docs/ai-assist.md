# AI Assist

AI assist is optional. The default provider is `noop`, and deterministic artifacts do not depend on AI output.

AI output must use the `.ai.md` suffix and must include a review warning.

## No-op Provider

```bash
openexit assist summarize \
  --project ./demo \
  --provider noop \
  --out ./demo/assessment/executive-summary.ai.md
```

The no-op provider does not call a network service. It produces a deterministic placeholder so the assist pipeline can be tested safely.

## LiteLLM Provider

External assist providers require explicit project opt-in:

```yaml
policy:
  allowAI: true
assist:
  enabled: true
  provider: litellm
  model: qwen
  allowExternalProvider: true
```

Set the LiteLLM endpoint through environment variables:

```bash
export OPENEXIT_LITELLM_BASE_URL=http://localhost:4000
export OPENEXIT_LITELLM_API_KEY=optional-token
```

Then run:

```bash
openexit assist summarize \
  --project ./demo \
  --provider litellm \
  --model qwen \
  --input ./demo/assessment/openexit.assessment.yaml \
  --out ./demo/assessment/executive-summary.ai.md \
  --save-redacted-input
```

OpenExit redacts the input before sending it. With `--save-redacted-input`, the exact redacted request payload is saved next to the `.ai.md` output for audit.

The assist command refuses to overwrite existing `.ai.md` outputs or saved audit inputs.
