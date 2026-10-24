# nlq — Invoice Keeper NL-query feature (Python)

Read-only assistant for "ask a question about payments". One Python project, one
image, three commands:

| Command | What it is |
|---|---|
| `python -m nlq.mcp_server` | FastMCP server, Streamable HTTP (stateless), 4 read-only tools + `/healthz` |
| `python -m nlq.agent` | agent loop over OpenRouter (FastAPI on :8090): `/v1/ask`, `/v1/ask/stream` (SSE), `/healthz` |
| `python -m nlq.dbinit` | idempotent creation of the `nlq_reader` role and column-level grants |

Design and rationale live in `docs/NL_QUERIES.md` (the "Решения пользователя"
table there overrides the rest of that document).

## Status

**Stage 1** (done): package skeleton, config, common helpers (actor-JWT
verification, timezone-aware "today"/period resolution, money/currency
formatting, sanitise, tool-spec single source of truth), `nlq.dbinit`, the MCP
server (4 tools, SQL adapters, visibility policy, limits, server-side period
resolution, fail-closed startup self-checks, periodic schema recheck,
`/healthz`), `nlq.devtoken`, `nlq.keygen`, `nlq.health`, Dockerfile.

**Stage 2** (this drop): the agent (`nlq.agent`). FastAPI app with the bounded
agent loop (§7.5), the OpenRouter client with one transient retry + manual
backup-model fallback + rules fallback (§7.2–7.3), a per-request MCP client over
Streamable HTTP forwarding the signed actor token, the Russian system prompt
(§7.4), code-built blocks/notes/followups (§7.7), the grounding check
(§7.7 p.3), the response envelope (§7.9), three brains (`openrouter` / `rules` /
`fake`), an SSE streaming endpoint (`/v1/ask/stream`) plus the non-streaming
`/v1/ask`, and `/healthz` with the model self-check. Eval set + runner in
`eval/`.

Not yet: the Go endpoint, the bot `/ask`, and the compose services — those come
later (another workflow owns `backend/`).

## Agent (stage 2)

```bash
# openrouter mode (real LLM): needs a key and a reachable MCP server
export OPENROUTER_API_KEY=sk-or-...
export NLQ_ACTOR_PUBLIC_KEY=...        # from nlq.keygen (same pair the Go signer uses)
export NLQ_MCP_URL=http://localhost:8900/mcp
export NLQ_LLM=openrouter              # or: rules (offline), fake (scripted)
uv run python -m nlq.agent             # serves http://0.0.0.0:8090

# drive it with a dev token (local only)
TOKEN=$(uv run python -m nlq.devtoken --role user)
curl -s localhost:8090/v1/ask -H "Authorization: Bearer $TOKEN" \
     -d '{"question":"что просрочено?","channel":"web"}' | jq

# SSE streaming (step -> block -> text -> done frames)
curl -N localhost:8090/v1/ask/stream -H "Authorization: Bearer $TOKEN" \
     -d '{"question":"что просрочено?","channel":"web"}'
```

Modes: `openrouter` is the production brain; `rules` is the deterministic
offline router (also the automatic fallback when the LLM is unavailable and
`NLQ_RULES_FALLBACK=true`); `fake` replays a JSON script named by
`NLQ_FAKE_SCRIPT` for e2e smoke tests.

Budgets (env, §7.6): `NLQ_MAX_LLM_CALLS=4`, `NLQ_MAX_TOOL_CALLS_PER_STEP=3`,
`NLQ_STEP_TIMEOUT=10`, `NLQ_TOOL_TIMEOUT=5`, `NLQ_TOTAL_TIMEOUT=22`,
`NLQ_MAX_TOKENS=700`. The budget is enforced with an absolute deadline: each LLM
attempt is capped at `min(step_timeout, time left)`, the backup model only runs
if budget remains, and the whole loop is wrapped in `asyncio.timeout(total)`
leaving ~2.5 s for the rules/template fallback so the Go 25 s timeout never fires
first. `/healthz` also probes the MCP server's `/healthz` every 15 s and reports
`mcp: ok|degraded|down` (status goes `degraded` in openrouter mode when MCP is not
ok), instead of always claiming MCP is up.

## Eval

```bash
uv run python eval/run_eval.py --mode rules          # offline routing check, no DB
# against a running agent (any brain); set the agent's NLQ_LLM + key to pick one:
NLQ_ACTOR_PRIVATE_KEY=... uv run python eval/run_eval.py --mode http --runs 3
```

`eval/questions.yaml` holds the 20 demo questions (§12.4) with the expected tool
and key arguments. Admission: 100% on questions 1–8 and ≥ 90% overall, per
model, across 3 runs. The CI test `tests/unit/test_eval_rules.py` runs the rules
subset offline so a routing regression fails fast.

## Quick start (local)

```bash
cd nlq
uv sync

# 1. Generate an Ed25519 keypair for actor-JWT (Go signs, Python verifies).
uv run python -m nlq.keygen        # prints NLQ_ACTOR_PRIVATE_KEY / NLQ_ACTOR_PUBLIC_KEY

# 2. Create the read-only role + grants, and local dev fixtures for B's tables.
export ADMIN_DSN=postgres://postgres:postgres@localhost:5432/invoice_keeper
export NLQ_DB_PASSWORD=choose-a-password
export APP_ENV=local NLQ_DEV_SCHEMA=nlq_dev
uv run python -m nlq.dbinit
# later, when B merges real tables: unset NLQ_DEV_SCHEMA and run:
# uv run python -m nlq.dbinit --drop-dev

# 3. Run the MCP server.
export NLQ_ACTOR_PUBLIC_KEY=...     # from keygen
export NLQ_DB_DSN=postgres://nlq_reader:$NLQ_DB_PASSWORD@localhost:5432/invoice_keeper
export NLQ_DEV_SCHEMA=nlq_dev
uv run python -m nlq.mcp_server     # serves http://0.0.0.0:8900/mcp and /healthz

# 4. Mint a dev token and poke it with MCP Inspector (local only).
export NLQ_ACTOR_PRIVATE_KEY=...    # from keygen
uv run python -m nlq.devtoken --role admin
```

`nlq.devtoken` is refused when `APP_ENV=production`.

## Models (OpenRouter)

Defaults (`NLQ_MODELS`), verified present on
`https://openrouter.ai/api/v1/models?supported_parameters=tools` on 2026-10-24,
both with `tools` + `tool_choice`, from different vendors:

| Role | id | ~price in/out per 1M | Why |
|---|---|---|---|
| primary | `anthropic/claude-sonnet-4.5` | $3 / $15 | reliable tool-calling, strong Russian |
| backup | `google/gemini-2.5-flash` | $0.15 / $1.25 | cheap, fast, different vendor → independent failure |

The final choice is gated by the eval set (§12.4); swap ids in `NLQ_MODELS`
without code changes. Both ids were re-verified present with `tools` +
`tool_choice` on the live endpoint on 2026-10-24 (376 tool-capable models
listed). Other confirmed-present, tool-capable candidates for the eval, from
other vendors: `openai/gpt-5.1` ($0.625 / $5 per 1M) and
`deepseek/deepseek-chat` (~$0.26 / $1.03 per 1M). The agent sends
`provider.require_parameters=true` + `data_collection:"deny"`, flat schemas,
`temperature=0`, `usage.include=true`, and a pseudonymous
`user = sha256(user_id)[:16]` (§7.1, §8.5).

## Quality gates

```bash
cd nlq
uv run ruff check .
uv run pyright
uv run pytest -q          # DB tests skip unless TEST_DATABASE_URL is set
```

DB-backed tests expect `TEST_DATABASE_URL` to point at a **superuser**
connection (they build the real `nlq_reader` role and connect as it). They are
**destructive** (they `TRUNCATE public.users`/`personas` and reset the
`nlq_reader` password), so they refuse to run unless the database name ends with
`_test`, or `NLQ_ALLOW_DESTRUCTIVE_TESTS=1` is set. Point them at a throwaway
database, e.g.:

```bash
docker run -d --rm --name pg-test -e POSTGRES_PASSWORD=pw -e POSTGRES_USER=pg \
  -e POSTGRES_DB=nlq_test -p 55439:5432 postgres:16
TEST_DATABASE_URL=postgres://pg:pw@localhost:55439/nlq_test uv run pytest -q
```

## Security notes

- Tools only ever run in a READ ONLY, repeatable-read transaction under a role
  with column-level `SELECT` grants (no `login`, `password_hash`, tokens, or
  telegram columns — ever). Startup self-checks fail closed.
- Identity comes only from a verified Ed25519 actor-JWT, never from tool
  arguments. The MCP server holds the public key only.
- Postgres error text, tokens and PII are never logged at info level.

### Known deviation

Unknown tool arguments (e.g. `user_id`, `scope`) are rejected by the MCP layer
itself (FastMCP publishes `additionalProperties:false`), surfacing as a tool
error rather than an `{"ok":false,"error":"bad_args"}` envelope. Cross-field
problems (amount without currency, bad custom dates, forbidden assignee) do
return the envelope. The agent treats a tool error as `bad_args`.
Python is pinned to 3.14 in the Dockerfile (env default) vs 3.12 in the spec;
`requires-python` stays `>=3.12`.

### Stage-2 deviations / notes

- **SSE emits post-grounding text in one frame.** The decisions table requires
  SSE to the client, so `/v1/ask/stream` exists and streams live `step` frames
  as tools run, then `block` frames and a single `text` frame followed by
  `done`. Per §9.4 the text is only sent after the grounding check, so it is not
  token-streamed; true token streaming stays "потом". Good enough for the demo
  and for the Go/nginx passthrough (`X-Accel-Buffering: no` is set).
- **"Сколько в месяц" = normalized monthly run-rate (decision #2).**
  `sum_occurrences` now has a `basis` argument. Without a named month, «сколько
  платим в месяц» uses `basis=run_rate`: the average monthly load per currency
  over active payments, normalized monthly ×1 / quarterly ÷3 / semiannual ÷6 /
  annual ÷12. B exposes no periodicity column yet, so the interval is **inferred**
  from the average gap between consecutive occurrence due_dates (`run_rate.sql`);
  single-occurrence payments default to monthly. When «в этом месяце», «в
  октябре», «в ноябре» name a month, `basis=period_sum` sums the occurrences due
  in that window. Either way the answer always states what was computed (a note
  built in `render.build_notes`), as the decision requires. When B ships a real
  interval column, only `run_rate.sql` changes.
- **Payment categories (decision #3).** `find_recurring_payments`,
  `find_occurrences` and `sum_occurrences` accept an optional `category` filter.
  B's `recurring_payments.category` column may not exist yet, so the startup
  schema probe detects it (`information_schema`): when present it is read and
  filtered; when absent the filter is dropped and the tool returns
  `category_unavailable:true` so the renderer adds a note, rather than silently
  returning nothing. The dev fixture (`db/dev_b_tables.sql`) ships the column for
  local/test use.
- **Coverage is per active payment (decision #4).** `horizon.sql` returns the
  earliest last-occurrence across active payments (not a single global
  `max(due_date)`) plus `any_without_periods`, so one payment generated far ahead
  cannot hide others whose periods were never created. `find_occurrences` and
  `sum_occurrences` both report `coverage_until`/`coverage_incomplete`, and the
  renderer says «периоды после ДД.ММ.ГГГГ ещё не созданы» instead of a bare «не
  найдено».
- **Unknown tool args** from the model are caught locally by the pydantic
  re-validation (`tool_specs`) and returned to the model as `bad_args` before
  any MCP round-trip.
- **Greeting / out-of-scope works when MCP is down**: `reply_without_data` needs
  no data, so the loop tolerates a failed MCP connect and only data tool calls
  then return `db_unavailable`.
- **`fake` script format is JSON** (not YAML) at runtime via `NLQ_FAKE_SCRIPT`,
  to keep PyYAML a dev-only dependency; tests build scripts in memory.
