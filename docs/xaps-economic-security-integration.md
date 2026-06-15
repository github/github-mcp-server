# Optional: XAPS Pre-Execution Audit for MCP Hosts

This guide shows how MCP host authors can add an **optional** economic-security layer before agents execute high-stakes actions (payments, contract calls, token transfers).

It is a **community integration pattern**, not a change to the GitHub MCP Server itself. Host applications wrap tool execution with a pre-flight audit and halt on rejection.

> **Not affiliated with GitHub.** XAPS is an independent protocol/SDK maintained at [APMC1/xaps-sdk](https://github.com/APMC1/xaps-sdk).

---

## When to use this

Use this pattern when your MCP host or agent runtime can trigger actions with real cost or liability — for example:

- Smart-contract writes from agent tool calls
- Payment or billing APIs invoked by autonomous workflows
- Any tool path where prompt injection or goal drift could cause unintended execution

Standard Web3 tooling verifies signatures. XAPS adds an independent pre-execution audit step and returns a receipt your host can log or require downstream.

---

## Install the SDK

The SDK is published from GitHub (not PyPI yet):

```bash
pip install "xaps @ git+https://github.com/APMC1/xaps-sdk.git@v0.1.1"
```

Set credentials:

```bash
export XAPS_AGENT_KEY="your_agent_key"
# Optional override (default in SDK examples may differ):
export XAPS_API_URL="https://api.xaps.network"
```

Docs: https://github.com/APMC1/xaps-sdk#readme

---

## Python wrapper pattern

The snippet below is self-contained. Adapt it to your host's tool-dispatch layer.

```python
import os
from typing import Any

from xaps import XapsClient, XapsRejectedError


class XapsSecureMCPWrapper:
    """Wrap MCP tool calls with Xaps economic security audit."""

    def __init__(self, agent_key: str | None = None):
        self.client = XapsClient(
            api_key=agent_key or os.getenv("XAPS_AGENT_KEY"),
            base_url=os.getenv("XAPS_API_URL", "https://api.xaps.network"),
        )

    def audit_before_execute(
        self,
        action: str,
        contract_address: str,
        amount: float,
        payload_override: dict[str, Any] | None = None,
    ) -> dict[str, Any]:
        receipt = self.client.audit(
            action=action,
            contract_address=contract_address,
            amount=amount,
            payload_override=payload_override,
        )

        audit = receipt["audit"]
        if audit["status"] == "REJECTED":
            raise XapsRejectedError(
                reason=audit["beta_attack"],
                receipt=receipt,
            )

        return receipt

    def health_check(self) -> bool:
        return self.client.health()


if __name__ == "__main__":
    security = XapsSecureMCPWrapper()
    try:
        receipt = security.audit_before_execute(
            action="transfer_tokens",
            contract_address="0x742d35Cc6634C0532925a3b844Bc9e7595f0bEb",
            amount=1000.0,
        )
        print(f"Approved: {receipt['audit']['alpha_defense']}")
    except XapsRejectedError as e:
        print(f"Rejected: {e.reason}")
```

---

## Integration sketch

1. Intercept the tool call in your MCP host **before** side effects.
2. Call `audit_before_execute(...)` with action metadata.
3. On `APPROVED`, proceed and optionally attach the receipt to logs/traces.
4. On `REJECTED`, return a tool error to the agent — do not execute.

This keeps the GitHub MCP Server unchanged; security lives in the host or a sidecar wrapper.

---

## Maintainer note

Per [CONTRIBUTING.md](../CONTRIBUTING.md), consider opening an issue first if you want this pattern linked from official docs or toolsets. This PR adds a standalone optional guide under `docs/` with no server code changes.