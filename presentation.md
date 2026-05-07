---
title: Bob the Tracker
slideOptions:
  theme: white
  transition: slide
---

# Bob the Tracker

> A dataset request tracking system for the **Future Circular Collider (FCC)** community at CERN.

Physics groups need large datasets produced centrally — for detector design, analysis, simulation.
Before Bob: emails, spreadsheets, Mattermost threads. Things fell through the cracks.

**Bob is a single place where anyone can submit a dataset request and know it will be handled.**

---

## The Workflow

```
Requester                   Manager
   │                           │
   │  Submit request           │
   │──────────────────────────►│
   │                           │  Assign + review
   │  ◄── status updates ──────│
   │                           │  Approve / Reject
   │                           │
   │                           │  In Progress → Completed
   │  ◄── email notification ──│
```

**Status lifecycle:** Draft → Pending Review → Approved → In Progress → Completed

Each step is logged in a per-request activity timeline with comments, internal notes, and system events.

---

## Key Features

| For Requesters | For Managers |
|---|---|
| HEP-specific fields (stage, use case, format, tags) | Pipeline view with batch approve / reject / complete |
| Markdown + LaTeX math in titles & descriptions | Inline status and priority overrides |
| Track own requests; add comments | Assign requests; leave internal notes |
| Filter & search by status, priority, text | Dashboard alerts for Critical-priority items |

**Bento-style dashboard** — live stats: total, pending, in-progress, completed.

Responsive design. Dark / light / system theme.

---

## Tech Stack

| Layer | Technology |
|---|---|
| Backend | Go 1.22+ (`net/http`, no framework) |
| Frontend | HTMX 2 + Tailwind CSS |
| Database | SQLite (no CGO) |
| Auth | CERN SSO via OpenID Connect (Keycloak) |
| Math / Markdown | KaTeX + marked.js |
| Email | Go stdlib `net/smtp` |

**Single self-contained binary.** All JS/CSS assets self-hosted — no external CDN at runtime.

----

## Getting Started

```bash
# Local dev — no CERN account needed
git clone https://github.com/kjvbrt/bob && cd bob
DEV_MODE=TRUE go run ./cmd/bob
# → open http://localhost:5050, pick a username and role
```

```bash
# Production (CERN SSO)
export OIDC_CLIENT_ID=...      OIDC_CLIENT_SECRET=...
export OIDC_REDIRECT_URL=https://your-host/auth/callback
export MANAGER_USERNAMES=jsmith,adoe
go build -o bob ./cmd/bob && ./bob
```

---

## Thank You

**github.com/kjvbrt/bob**

Questions & issues → GitHub Issues
MC Production team → FCC-PED-SoftwareAndComputing-MCProduction@cern.ch
