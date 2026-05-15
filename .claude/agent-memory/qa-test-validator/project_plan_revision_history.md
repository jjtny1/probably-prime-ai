---
name: plan-revision-history
description: This project's plan file keeps old narrative when the API shape changes; the Revision History at the bottom is authoritative.
metadata:
  type: project
---

Fact: `.claude/docs/2026-05-14-x402-prime-api.md` Section C/D still uses `GET /prime?bits=N` in the narrative even though the API was switched to path-based `GET /prime/{bits}`. The Revision History entry dated 2026-05-15 records the Option A fix (path-based tiers, 4 routes, per-tier prices).

**Why:** The planner did not rewrite the narrative when the developer applied the Option A defect fix; only the Revision History was appended. README and code reflect the new shape correctly.

**How to apply:** When auditing this plan, treat the latest Revision History entry as authoritative for API shape. Cross-reference README + code + test files (all path-based) over the narrative. Flag the narrative drift as a documentation concern but not a blocking failure if README + code + tests are consistent.
