"""QuickJS in-memory sandbox with hard timeout (process kill on deadlock)."""

from __future__ import annotations

import json
import logging
import multiprocessing as mp
from typing import Any

logger = logging.getLogger(__name__)


class SandboxError(RuntimeError):
    """Raised when sandbox evaluation fails or times out."""


_DEFAULT_WRAPPER = """
(function(input) {
  var __result = (function(raw) {
    %s
  })(input);
  if (typeof __result === 'string') {
    return __result;
  }
  return JSON.stringify(__result);
})
"""


def _eval_sync(js_body: str, raw_input: str) -> str:
    import quickjs

    ctx = quickjs.Context()
    wrapped = _DEFAULT_WRAPPER % js_body
    fn = ctx.eval(wrapped)
    out = fn(raw_input)
    if out is None:
        raise SandboxError("sandbox returned null")
    return str(out)


def _process_main(conn: Any, js_body: str, raw_input: str) -> None:
    """Child process entry: evaluate JS and send result over a pipe."""
    try:
        rendered = _eval_sync(js_body, raw_input)
        conn.send(("ok", rendered))
    except Exception as exc:  # noqa: BLE001
        conn.send(("err", f"{type(exc).__name__}: {exc}"))
    finally:
        conn.close()


def run_js(
    js_body: str,
    raw_input: str,
    *,
    timeout_seconds: float = 2.0,
) -> list[Any]:
    """Evaluate rule JS in QuickJS and parse a JSON array result.

    Runs in a child process so infinite loops can be terminated on timeout
    (SPEC §6.4 / docs thinking tip: prevent QuickJS dead-loops).
    """
    if not js_body or not js_body.strip():
        raise SandboxError("js_body is empty")
    timeout = max(0.1, float(timeout_seconds))

    ctx = mp.get_context("fork")
    parent_conn, child_conn = ctx.Pipe(duplex=False)
    proc = ctx.Process(target=_process_main, args=(child_conn, js_body, raw_input))
    proc.daemon = True
    proc.start()
    child_conn.close()

    try:
        if parent_conn.poll(timeout):
            status, payload = parent_conn.recv()
        else:
            proc.terminate()
            proc.join(1.0)
            if proc.is_alive():
                proc.kill()
                proc.join(1.0)
            raise SandboxError(f"sandbox timed out after {timeout}s")
    finally:
        parent_conn.close()
        if proc.is_alive():
            proc.terminate()
            proc.join(1.0)
        else:
            proc.join(1.0)

    if status == "err":
        raise SandboxError(f"sandbox eval failed: {payload}")

    rendered = payload
    try:
        data = json.loads(rendered)
    except json.JSONDecodeError as exc:
        raise SandboxError(f"sandbox output is not JSON: {rendered[:200]}") from exc

    if not isinstance(data, list):
        raise SandboxError(f"sandbox output must be a JSON array, got {type(data).__name__}")
    return data
