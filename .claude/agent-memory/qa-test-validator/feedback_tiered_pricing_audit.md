---
name: tiered-pricing-audit
description: When tests assert "RoutesConfig is non-empty" but spec says "one entry per tier with tier price", count entries and check per-tier prices explicitly.
metadata:
  type: feedback
---

Rule: For any plan that defines N distinct pricing tiers wired through an SDK that route-matches by path/key, the QA audit MUST verify `len(routes) == N` AND `routes[key].price == tier(bits)` for each tier — not merely "compiles" or "Accepts non-empty".

**Why:** A tiered-pricing defect reached on-chain because earlier QA passes accepted a weakened Test 6.1/6.2 that asserted a single route with one hardcoded price. The SDK matches on path only, so every bit size paid the cheapest tier. Two QA cycles missed it because the tests existed by name and passed.

**How to apply:**
- When the plan names tiers, grep `BuildRoutesConfig`-equivalent test for `len(routes)` and per-key price equality.
- If a test comment justifies a relaxation by claiming "the SDK only supports X", do not accept it — re-read the SDK source or escalate to the planner. Comments can lie; spec cannot be weakened.
- Use Phase 4a (Test Body Audit) and Phase 4b (Full-plan scope) — never narrow to "just the latest diff".
