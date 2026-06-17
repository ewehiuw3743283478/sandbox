// Optional jshandler-compatible script version.
// Use this only if you install router-for-me/cpa-plugin-jshandler manually
// and mount this script into the CLIProxyAPI container, then set jshandler.script_paths.

function on_after_auth_request(ctx) {
  return forceCodexWebSearch(ctx);
}

function on_before_request(ctx) {
  // Usually leave before-auth untouched because CLIProxyAPI may not have selected
  // the upstream/ToFormat yet. Uncomment the next line if you intentionally want that.
  // return forceCodexWebSearch(ctx);
  return ctx;
}

function forceCodexWebSearch(ctx) {
  if (!ctx || !ctx.body) return ctx;
  var toFormat = String(ctx.toFormat || ctx.to_format || '').toLowerCase();
  var sourceFormat = String(ctx.sourceFormat || ctx.source_format || ctx.protocol || '').toLowerCase();
  if (toFormat !== 'codex' && sourceFormat !== 'codex') return ctx;

  var body;
  try {
    body = JSON.parse(ctx.body);
  } catch (_) {
    return ctx;
  }
  if (!body || typeof body !== 'object' || !Object.prototype.hasOwnProperty.call(body, 'input')) return ctx;

  if (!Array.isArray(body.tools)) body.tools = [];
  var found = false;
  for (var i = 0; i < body.tools.length; i++) {
    if (body.tools[i] && body.tools[i].type === 'web_search') {
      found = true;
      break;
    }
  }
  if (!found) {
    body.tools.push({ type: 'web_search', search_context_size: 'medium' });
  }
  body.tool_choice = 'required';
  body.parallel_tool_calls = true;
  body.max_tool_calls = Math.max(Number(body.max_tool_calls || 0), 4);

  if (!Array.isArray(body.include)) body.include = [];
  if (body.include.indexOf('web_search_call.action.sources') < 0) {
    body.include.push('web_search_call.action.sources');
  }

  var instruction = 'For any question that may require current, external, API, package, repository, changelog, security, pricing, policy, or source-grounded information, use the hosted web_search tool before answering. Prefer official documentation and primary sources. Include citations when the response format supports them.';
  if (typeof body.instructions === 'string' && body.instructions.indexOf(instruction) < 0) {
    body.instructions += '\n\n' + instruction;
  } else if (typeof body.instructions !== 'string') {
    body.instructions = instruction;
  }

  ctx.body = JSON.stringify(body);
  return ctx;
}
