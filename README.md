# FCC Dataset Request System

A web-based dataset request tracking system for centrally produced datasets
used in analysis, detector design studies, and other physics related areas at
the [Future Circular Collider (FCC)](https://fcc.web.cern.ch/).

Bob keeps an eye on the dataset needs of the community around the FCC and makes
sure nothing falls through the cracks.

<p align="center"><img src="static/logo.png" alt="FCC Dataset Request System logo" width="160"/></p>

---

## Features

- **Submit requests** for FCC datasets with HEP-specific fields (dataset stage, use case, format, tags)
- **Track status** through the full pipeline: Draft → Pending Review → Approved → In Progress → Completed
- **Priority levels** — Low, Medium, High, Critical — with dashboard alerts for critical requests
- **Pipeline view** for managers: assignment, batch actions, inline status and priority overrides
- **Activity log** per request: comments, internal manager notes, system events with timestamps
- **Assignment** of requests to managers with dropdown selector
- **Batch actions** — approve, reject, complete, or move to in-progress across multiple requests at once
- **Filter & search** by status, priority, or free text
- **Bento-style dashboard** with live stats (total, pending, in-progress, completed)
- **Markdown + LaTeX math** rendering in titles and descriptions (KaTeX + marked.js)
- **Email notifications** on status changes (optional, via SMTP)
- **Dark / light / system** theme with persistent preference
- **Responsive design** — works on desktop and mobile

---

## Tech Stack

| Layer     | Technology                                                              |
|-----------|-------------------------------------------------------------------------|
| Backend   | Go 1.22+ (`net/http` standard library)                                  |
| Frontend  | HTMX 2 + Tailwind CSS (self-hosted, compiled)                           |
| Database  | SQLite (local dev, no CGO) / PostgreSQL via CERN DBOD (production)      |
| Auth      | CERN SSO via OpenID Connect (Keycloak)                                  |
| Math/MD   | KaTeX + marked.js (self-hosted)                                         |
| Fonts     | Inter (self-hosted woff2 subsets)                                       |
| Email     | Go stdlib `net/smtp`                                                    |

Single binary, no CGO required. No external CDN dependencies at runtime.

---

## Getting Started

### Prerequisites

- Go 1.22 or later

### Local development (dev mode)

CERN SSO is bypassed in dev mode. A simple form lets you pick any username and role — no credentials required.

```bash
git clone https://github.com/kjvbrt/bob
cd bob
DEV_MODE=TRUE go run ./cmd/bob
```

Open **http://localhost:5050**, enter a username, choose a role (requester or manager), and log in.

### Build

```bash
# Development build (SQLite)
go build -o bob ./cmd/bob

# Production build (PostgreSQL — for CERN PaaS deployment)
go build -tags prod -o bob ./cmd/bob
```

### Front-end assets

All JS/CSS dependencies (HTMX, Tailwind CSS, KaTeX, marked, Inter font) are self-hosted under `static/vendor/`. To download them and regenerate the compiled Tailwind CSS:

```bash
./scripts/download-assets.sh
```

Run this once after cloning and again whenever you modify templates (Tailwind only includes classes it finds in the templates).

> **TODO:** Replace the Tailwind CLI build step with hand-crafted utility classes in `style.css`, eliminating the build dependency entirely.

The server starts on **http://localhost:5050**. The SQLite database is created automatically at `./data/requests.db` on first run (dev mode only).

### Email notifications (optional)

Set the following environment variables to enable email notifications on status changes:

```bash
export SMTP_HOST=smtp.cern.ch
export SMTP_PORT=587
export SMTP_USER=your-username
export SMTP_PASS=your-password
export SMTP_FROM=bob@cern.ch
```

If not set, email is silently disabled.

---

## Authentication & Roles

Authentication uses **CERN SSO** (Keycloak / OpenID Connect). Requester identity (name, username, email) is always taken from SSO and cannot be edited by users.

| Role          | Permissions |
|---------------|-------------|
| **Requester** | Submit requests, view all requests, edit own requests while draft or pending, add comments |
| **Manager**   | Everything above + change status/priority on any request, assign requests, delete requests, batch actions, internal notes |

Role assignment:
- CERN usernames listed in `MANAGER_USERNAMES` receive the **manager** role on first login
- All other authenticated users receive the **requester** role
- Roles are stored in the local database and persist across logins

---

## Project Structure

```
bob/
├── cmd/
│   └── bob/
│       └── main.go               # Server entry point, routing
├── internal/
│   ├── auth/
│   │   └── oidc.go               # CERN SSO OIDC client
│   ├── db/
│   │   ├── db.go                 # DB wrapper (Rebind, Like helpers)
│   │   ├── sqlite.go             # SQLite init & migrations (dev, build tag: !prod)
│   │   └── postgres.go           # PostgreSQL init & migrations (prod, build tag: prod)
│   ├── email/
│   │   └── email.go              # SMTP email notifications
│   ├── middleware/
│   │   └── auth.go               # Session middleware, role guards
│   ├── models/
│   │   ├── helper.go             # Driver-aware query helpers & time scanner
│   │   ├── request.go            # Dataset request model & store
│   │   ├── update.go             # Activity log model & store
│   │   └── user.go               # User & session model & store
│   └── handlers/
│       ├── handlers.go           # HTTP handlers & template rendering
│       ├── auth.go               # Login, callback, logout, dev login
│       └── pipeline.go           # Manager pipeline handlers
├── templates/
│   ├── layout.html               # Base layout (nav, modal, theme, footer)
│   ├── login.html                # Login page
│   ├── index.html                # Dashboard
│   ├── requests.html             # Request list with filters
│   ├── manager.html              # Manager pipeline view
│   └── partials/                 # HTMX-swappable fragments
│       ├── stats_cards.html
│       ├── request_list.html
│       ├── request_form.html
│       ├── request_detail.html
│       ├── events.html           # Activity log + comment form
│       ├── assignment.html       # Manager assignment dropdown
│       ├── priority_cell.html    # Inline priority select
│       ├── batch_toolbar.html    # Batch action toolbar
│       └── status_badge.html
├── static/
│   ├── style.css                 # Custom styles (bento grid, badges, dark mode)
│   ├── logo.png
│   └── favicon.png
├── openshift/                    # CERN PaaS deployment manifests
│   ├── secret.yaml               # OIDC + DB credentials (fill in before applying)
│   ├── configmap.yaml            # Manager usernames
│   ├── deployment.yaml           # Deployment spec
│   ├── service.yaml              # ClusterIP service
│   └── route.yaml                # HTTPS route (*.web.cern.ch)
├── Dockerfile                    # Multi-stage production image (-tags prod)
├── data/                         # SQLite database (dev only, git-ignored)
├── go.mod
├── go.sum
├── LICENSE
└── README.md
```

---

## Deployment (CERN PaaS / OpenShift)

Bob runs on [CERN PaaS](https://paas.cern.ch) (OpenShift) with a PostgreSQL database provided by the [CERN DBOD](https://dbod.web.cern.ch) service.

### Prerequisites

- A CERN OpenShift project (`oc login` access)
- A PostgreSQL instance provisioned via CERN DBOD
- Application registered at the [CERN Application Portal](https://application-portal.web.cern.ch) to obtain an OIDC client ID and secret

### 1. Build and push the container image

The production build uses the `prod` build tag, which enables PostgreSQL and disables SQLite.

```bash
docker build -t gitlab-registry.cern.ch/<your-group>/bob:latest .
docker push gitlab-registry.cern.ch/<your-group>/bob:latest
```

Update `openshift/deployment.yaml` with your image path.

### 2. Fill in secrets

Edit `openshift/secret.yaml` with your actual credentials (do **not** commit this file with real values):

```yaml
stringData:
  database-url: "postgresql://user:password@dbod-host.cern.ch:5432/database?sslmode=require"
  oidc-client-id: "your-client-id"
  oidc-client-secret: "your-client-secret"
  oidc-redirect-url: "https://bob.web.cern.ch/auth/callback"
```

Edit `openshift/configmap.yaml` with the CERN usernames that should have the manager role:

```yaml
data:
  manager-usernames: "jsmith,adoe"
```

Edit `openshift/route.yaml` and replace `REPLACE_WITH_HOSTNAME` with your actual hostname (e.g. `bob.web.cern.ch`).

### 3. Apply the manifests

```bash
oc apply -f openshift/secret.yaml
oc apply -f openshift/configmap.yaml
oc apply -f openshift/deployment.yaml
oc apply -f openshift/service.yaml
oc apply -f openshift/route.yaml
```

### 4. Verify

```bash
oc get pods        # pod should reach Running state
oc get route bob   # shows the public URL
oc logs -f deployment/bob  # tail logs
```

The database schema is created automatically on first startup. No manual migration step is required.

### Environment variables reference

| Variable            | Required | Description |
|---------------------|----------|-------------|
| `DATABASE_URL`      | Yes (prod) | PostgreSQL connection string from CERN DBOD |
| `OIDC_CLIENT_ID`    | Yes (prod) | CERN Application Portal client ID |
| `OIDC_CLIENT_SECRET`| Yes (prod) | CERN Application Portal client secret |
| `OIDC_REDIRECT_URL` | Yes (prod) | Must be `https://<hostname>/auth/callback` |
| `MANAGER_USERNAMES` | No  | Comma-separated CERN usernames granted manager role |
| `PORT`              | No  | HTTP listen port (default `5050`) |
| `DEV_MODE`          | No  | Set to `TRUE` to bypass CERN SSO (local dev only) |
| `SQLITE_PATH`       | No  | Override SQLite file path (dev only, default `./data/requests.db`) |
| `SMTP_HOST`         | No  | SMTP server for email notifications |
| `SMTP_PORT`         | No  | SMTP port (default 587) |
| `SMTP_USER`         | No  | SMTP username |
| `SMTP_PASS`         | No  | SMTP password |
| `SMTP_FROM`         | No  | From address for notification emails |

---

## Dataset Request Fields

| Field                      | Required | Description |
|----------------------------|----------|-------------|
| Title                      | Yes | Short description; supports Markdown + LaTeX math |
| Description                | No  | Physics process, energy range, detector concept, selection criteria |
| Use Case                   | No  | Physics Analysis, Reconstruction Development, Detector Simulation, ML Training/Evaluation, Benchmarking, Calibration |
| Dataset Stage              | No  | Generation, Simulation, Reconstruction, Other |
| Working Group / Team       | No  | e.g. Tracker WG, Calorimetry WG |
| Format Needed              | Yes | EDM4hep, HepMC3, ROOT, … |
| Statistics / Estimated Size| Yes | Number of events or file size |
| Due Date                   | No  | When the data is needed |
| Priority                   | No  | Low / Medium / High / Critical |
| Tags                       | No  | e.g. `fcc-hh`, `fcc-ee`, `higgs`, `top`, `bsm`, `llp` |
| Notes                      | No  | Generator settings, beam conditions, special requirements |

Requester identity (name, username, email) is populated automatically from CERN SSO and is not editable.

---

## ToDo

* Validation plots generation
* Dataset final stage
* Key4hep release
* MC Generator Card upload
* Extensions:
  * Extend the existing production (number of events)
  * Continue along the event processing chain
  * Extend number of targeted detectors
* Statistics and size should be separated
* Add Delphes as a stage
* Field to specify detector option and version


---

## Contact & Support

- **General questions**: [FCC-PED-SoftwareAndComputing-MCProduction@cern.ch](mailto:FCC-PED-SoftwareAndComputing-MCProduction@cern.ch)
- **Platform issues & feature requests**: [github.com/kjvbrt/bob/issues](https://github.com/kjvbrt/bob/issues)

---

## Acknowledgements

Built with the assistance of [Claude](https://claude.ai) (Anthropic).
