# DividedByZeroAgent

This agent encodes a simple rule: **division by zero collapses to the TEND state (0)**.

### Rationale
- In ternary logic: REFRAIN = -1, TEND = 0, AFFIRM = +1
- Normally, division by zero is an exception.
- Here, any ZeroDivisionError is treated as **0 → TEND (observe)**, preventing crash loops.

### Quickstart

```bash
python -m venv .venv
source .venv/bin/activate
pip install -U pip -r requirements.txt

# run
python -m divided_by_zero run --a 5 --b 0
```

Expected output: `{ "result": 0, "state": "TEND" }`
