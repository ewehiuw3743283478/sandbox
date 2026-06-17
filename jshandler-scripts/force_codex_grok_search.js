// Optional script for router-for-me/cpa-plugin-jshandler.
// The native plugin-store build in this repository is recommended for "paste URL into CLIProxyAPI".
// This JS version is a reference/manual fallback when the jshandler plugin is already installed.

function isResponsesBody(body) {
  return body && Object.prototype.hasOwnProperty.call(body, 'input');
}

function asLower(s) {
  return String(s || '').trim().toLowerCase();
}

function wildcard(pattern, value) {
  pattern = asLower(pattern);
  value = asLower(value);
  if (!pattern) return false;
  if (pattern === '*') return true;
  const parts = pattern.split('*');
  if (parts.length === 1) return pattern === value;
  if (!value.startsWith(parts[0])) return false;
  let pos = parts[0].length;
  for (let i = 1; i < parts.length - 1; i++) {
    const part = parts[i];
    if (!part) continue;
    const idx = value.indexOf(part, pos);
    if (idx < 0) return false;
    pos = idx + part.length;
  }
  const last = parts[parts.length - 1];
  return !last || value.endsWith(last);
}

function anyMatch(values, patterns) {
  return values.some((v) => patterns.some((p) => wildcard(p, v)));
}

function ensureTool(body, tool) {
  if (!Array.isArray(body.tools)) body.tools = [];
  const idx = body.tools.findIndex((t) => t && t.type === tool.type);
  if (idx >= 0) return false;
  body.tools.push(tool);
  return true;
}

function ensureInclude(body, value) {
  if (!Array.isArray(body.include)) body.include = [];
  if (body.include.includes(value)) return false;
  body.include.push(value);
  return true;
}

function addInstruction(body, instruction) {
  if (!instruction) return false;
  const existing = typeof body.instructions === 'string' ? body.instructions : '';
  if (existing.includes(instruction)) return false;
  body.instructions = existing.trim() ? `${existing}\n\n${instruction}` : instruction;
  return true;
}

function targetKind(ctx, body) {
  const formats = [ctx.source_format, ctx.sourceFormat, ctx.to_format, ctx.toFormat];
  const models = [ctx.model, body && body.model];
  if (anyMatch(formats, ['xai', 'x-ai', 'grok']) || anyMatch(models, ['grok*', 'grok-*', 'xai/*', 'x-ai/*'])) return 'grok';
  if (anyMatch(formats, ['codex']) || anyMatch(models, ['gpt-5*', 'gpt-4.1*', 'o3*', 'o4*'])) return 'codex';
  return '';
}

function mutate(ctx) {
  if (!ctx || !ctx.body) return ctx;
  let body;
  try {
    body = JSON.parse(ctx.body);
  } catch (_) {
    return ctx;
  }
  if (!isResponsesBody(body)) return ctx;

  const kind = targetKind(ctx, body);
  if (!kind) return ctx;

  let changed = false;
  if (kind === 'grok') {
    changed = ensureTool(body, { type: 'web_search' }) || changed;
    changed = ensureTool(body, { type: 'x_search' }) || changed;
    changed = addInstruction(
      body,
      'For current or source-grounded questions, use both web_search and x_search before answering when available. Prefer official documentation, primary sources, and relevant X posts. Include citations when supported.'
    ) || changed;
  } else {
    changed = ensureTool(body, { type: 'web_search', search_context_size: 'medium' }) || changed;
    changed = ensureInclude(body, 'web_search_call.action.sources') || changed;
    changed = addInstruction(
      body,
      'For current or source-grounded questions, use the hosted web_search tool before answering. Prefer official documentation and primary sources. Include citations when supported.'
    ) || changed;
  }

  if (body.tool_choice !== 'required') {
    body.tool_choice = 'required';
    changed = true;
  }
  if (body.parallel_tool_calls !== true) {
    body.parallel_tool_calls = true;
    changed = true;
  }
  if (body.max_tool_calls !== 6) {
    body.max_tool_calls = 6;
    changed = true;
  }

  if (changed) ctx.body = JSON.stringify(body);
  return ctx;
}

function on_after_auth_request(ctx) {
  return mutate(ctx);
}

function on_before_request(ctx) {
  return ctx;
}
