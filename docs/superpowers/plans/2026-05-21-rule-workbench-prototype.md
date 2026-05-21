# Rule Workbench Prototype Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a temporary, reviewable Rule creator workbench prototype before production implementation begins.

**Architecture:** Create a standalone static prototype under `prototypes/rule-workbench/` so it can be reviewed without touching production React routes, API types, database schema, or tests. The prototype models the approved workbench layout with source type/bank controls, exact sender chips, regex fields, email sample tabs, expected values, and live extraction results using JavaScript regex as a visual approximation only.

**Tech Stack:** Plain HTML, CSS, and JavaScript served from a local static server for review.

---

## File Structure

- Create `prototypes/rule-workbench/index.html`: complete standalone prototype with layout, styling, sample state, and interactions.
- No production code changes in this phase.
- No production tests in this phase, per the approved design.

## Visual Thesis

Dense operational workbench: dark, restrained, split-pane, with rule definition pinned on the left and the email body/results workspace given the majority of the screen.

## Content Plan

- Header: rule name, origin, save/export actions.
- Left panel: identity, source, match, extract.
- Right workspace: sample tabs, large email body, expected output, live extraction result.
- Footer/status: fixture filename preview and pass/fail summary.

## Interaction Thesis

- Sender emails are edited as removable chips plus an input.
- Sample tabs switch between email variations while keeping rule fields stable.
- Live extraction result updates immediately as regexes, expected fields, or sample body change.

---

### Task 1: Static Rules Prototype

**Files:**
- Create: `prototypes/rule-workbench/index.html`
- Create: `prototypes/rule-workbench/list.html`

- [x] **Step 1: Create the prototype file**

Use `apply_patch` to create `prototypes/rule-workbench/index.html` with:

```html
<!doctype html>
<html lang="en">
  <head>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <title>Rule Workbench Prototype</title>
    <style>
      :root {
        color-scheme: dark;
        --bg: #0b0d12;
        --panel: #11141b;
        --panel-soft: #151a23;
        --border: #252b37;
        --muted: #8d98aa;
        --text: #eef2f7;
        --accent: #8b5cf6;
        --accent-strong: #a78bfa;
        --green: #22c55e;
        --red: #fb7185;
        --yellow: #f59e0b;
        font-family:
          Inter, ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
      }

      * {
        box-sizing: border-box;
      }

      body {
        margin: 0;
        min-height: 100vh;
        background: var(--bg);
        color: var(--text);
      }

      button,
      input,
      textarea {
        font: inherit;
      }
    </style>
  </head>
  <body>
    <main id="app"></main>
    <script>
      const state = {
        rule: {
          name: "HDFC Credit Card",
          origin: "Predefined",
          sourceType: "credit_card",
          sourceLabel: "Credit Card",
          bank: "HDFC",
          senders: ["alerts@hdfcbank.bank.in", "alerts@hdfcbank.net"],
          subject: "Alert : Update on your HDFC Bank Credit Card",
          amountRegex: "Rs\\\\.\\\\s*([\\\\d,]+(?:\\\\.\\\\d+)?)",
          merchantRegex: "\\\\bat\\\\b (.*?) on",
          currencyRegex: ""
        },
        sampleIndex: 0,
        samples: [
          {
            name: "classic spend",
            sender: "HDFC Alerts <alerts@hdfcbank.net>",
            subject: "Alert : Update on your HDFC Bank Credit Card",
            body: "Dear Customer,\\nRs.999.00 spent at SWIGGY on your HDFC Credit Card on 12-Apr-2026.",
            want: { amount: "999.00", merchant: "SWIGGY", currency: "INR" }
          },
          {
            name: "standing instruction",
            sender: "HDFC Alerts <alerts@hdfcbank.bank.in>",
            subject: "Alert : Update on your HDFC Bank Credit Card",
            body: "Dear Customer,\\nRs. 149.00 spent at SPOTIFYINDIA on your HDFC Credit Card on 18-Apr-2026.",
            want: { amount: "149.00", merchant: "SPOTIFYINDIA", currency: "INR" }
          }
        ]
      };

      function render() {
        document.querySelector("#app").innerHTML = "<p>Prototype shell</p>";
      }

      render();
    </script>
  </body>
</html>
```

- [x] **Step 2: Expand the HTML/CSS/JS into the full workbench**

Replace the shell render with the final static UI:

- header with rule title, origin badge, fixture filename preview, and action buttons;
- two-column workbench layout;
- left panel groups for identity, source, match, and extract;
- right panel sample tabs, sample metadata, large body editor, expected values, and live result;
- JavaScript functions for exact sender parsing, subject contains, regex extraction, sample switching, sender chip editing, and fixture filename preview.

Keep all code in `index.html` because this is a throwaway prototype.

- [x] **Step 3: Serve the prototype**

Run:

```bash
python3 -m http.server 4177 --directory prototypes/rule-workbench
```

Expected: server starts and prints `Serving HTTP on`.

- [x] **Step 4: Verify in browser**

Open:

```text
http://localhost:4177
```

Verified manually through user review:

- The left panel remains visible while editing samples.
- Switching sample tabs updates body and expected values.
- Editing regex fields updates live result.
- Sender match uses exact parsed email address, not substring matching.
- The layout remains usable at desktop width and does not visibly overlap at narrow width.
- The list view is available at `/list.html`.
- The list columns are ordered Bank, Name, Subject, Senders, Type, Origin.
- Both prototype pages were accepted by the user for proceeding to production implementation.

- [x] **Step 5: Commit**

Run:

```bash
git add prototypes/rule-workbench docs/superpowers/plans/2026-05-21-rule-workbench-prototype.md docs/superpowers/specs/2026-05-21-rule-redesign-design.md
git commit --no-gpg-sign -m "prototype: add rule workbench"
```

Expected: commit succeeds.

---

## Self-Review

- Spec coverage: this plan covers the required prototype phase only. It intentionally does not implement production backend/frontend/docs because production implementation is gated on prototype approval.
- Placeholder scan: no TBD/TODO placeholders.
- Type consistency: prototype source fields use `sourceType`, `sourceLabel`, and `bank`, matching the approved source object semantics.
