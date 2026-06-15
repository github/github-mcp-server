"""
Xaps Security Integration Example for GitHub MCP Server
========================================================

This example shows how to add economic security to any MCP server
using the Xaps cognitive circuit breaker.

Before your agent executes a smart contract, transfer, or financial action,
it calls Xaps for an independent dual-agent audit.

Install (from GitHub — not on PyPI yet):
    pip install "xaps @ git+https://github.com/APMC1/xaps-sdk.git@v0.1.1"

Canonical doc for the github-mcp-server PR lives at:
    ../docs/xaps-economic-security-integration.md

Usage:
    Set XAPS_AGENT_KEY environment variable, then import this module
    in your MCP server setup.

Docs: https://github.com/APMC1/xaps-sdk
"""

import os
from typing import Any

from xaps import XapsClient, XapsRejectedError


class XapsSecureMCPWrapper:
    """Wraps MCP tool calls with Xaps economic security audit."""

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
        """
        Audit a financial action before execution.

        Raises XapsRejectedError if the audit rejects the action.
        Returns the receipt on approval.
        """
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
        """Check if Xaps API is reachable."""
        return self.client.health()


# Example integration with an MCP server tool
if __name__ == "__main__":
    # Initialize the security wrapper
    security = XapsSecureMCPWrapper()

    # Example: Before executing a token transfer
    try:
        receipt = security.audit_before_execute(
            action="transfer_tokens",
            contract_address="0x742d35Cc6634C0532925a3b844Bc9e7595f0bEb",
            amount=1000.0,
        )
        print(f"✅ Approved: {receipt['audit']['alpha_defense']}")
        print(f"💰 Balance remaining: ${receipt['remaining_balance']:.2f}")
        # Proceed with actual execution...
    except XapsRejectedError as e:
        print(f"❌ Rejected: {e.reason}")
        # Halt execution — do not proceed
    except Exception as e:
        print(f"⚠️ Audit failed: {e}")
        # Decide: halt or proceed based on your risk policy
