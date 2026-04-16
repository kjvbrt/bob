# Bob the Tracker

A web-based dataset request tracking system for analysis and detector design at the [Future Circular Collider (FCC)](https://fcc.web.cern.ch/).

Bob keeps an eye on your data needs — simulation samples, reconstruction outputs, detector ntuples — and makes sure nothing falls through the cracks.

![Bob the Tracker logo](static/logo.png)

---

## Features

- **Submit requests** for FCC datasets (MC simulation, reconstruction, raw detector data, analysis ntuples, etc.)
- **Track status** through the full pipeline: Draft → Pending Review → Approved → In Progress → Completed
- **Priority levels** — Low, Medium, High, Critical — with dashboard alerts for urgent requests
- **Inline status updates** via HTMX without full page reloads
- **Filter & search** by status, priority, or free text across the request list
- **Bento-style dashboard** with live stats and recent activity
- **FCC-aware fields**: working group, use case (detector design, ML training, calibration…), format (ROOT, EDM4hep, HDF5…), generator notes, beam conditions

---

## Tech Stack

| Layer    | Technology                          |
|----------|-------------------------------------|
| Backend  | Go 1.22 (`net/http` standard library) |
| Frontend | HTMX 2 + Tailwind CSS (CDN)        |
| Database | SQLite (`modernc.org/sqlite`)       |

No CGO required. Single binary deployment.

---

## Getting Started

### Prerequisites

- Go 1.22 or later

### Local development

CERN SSO can be bypassed with dev mode, which shows a simple login form where you pick any username and role — no credentials required.

```bash
git clone <repo-url>
cd bob
DEV_MODE=TRUE go run .
```

Open **http://localhost:5050**, select a username and role (requester or manager), and sign in.

### Production (CERN SSO)

Register the application at the [CERN Application Portal](https://application-portal.web.cern.ch) to obtain a client ID and secret, then:

```bash
export OIDC_CLIENT_ID=your-client-id
export OIDC_CLIENT_SECRET=your-client-secret
export OIDC_REDIRECT_URL=https://your-host/auth/callback
export MANAGER_USERNAMES=jsmith,adoe   # comma-separated CERN usernames

go run .
```

The server starts on **http://localhost:5050**. The SQLite database is created automatically at `./data/requests.db` on first run.

### Build

```bash
go build -o bob-tracker .
./bob-tracker
```

---

## Authentication & Roles

The app uses **CERN SSO** (Keycloak / OpenID Connect) for authentication. Two roles are supported:

| Role | Permissions |
|---|---|
| **Requester** | Submit requests, view all requests, edit own requests while draft or pending |
| **Manager** | Everything above + change status on any request, edit any request, delete requests |

Role assignment:
- CERN usernames listed in `MANAGER_USERNAMES` receive the **manager** role on their first login
- All other authenticated users receive the **requester** role
- Roles are stored in the local database and persist across logins

---

## Project Structure

```
.
├── main.go                   # Server entry point, routing
├── internal/
│   ├── auth/
│   │   └── oidc.go           # CERN SSO OIDC client
│   ├── db/
│   │   └── db.go             # SQLite init & migrations
│   ├── middleware/
│   │   └── auth.go           # Session middleware, role guards
│   ├── models/
│   │   ├── request.go        # Dataset request model & repository
│   │   └── user.go           # User & session model
│   └── handlers/
│       ├── handlers.go       # HTTP handlers & template rendering
│       └── auth.go           # Login, callback, logout, dev login
├── templates/
│   ├── layout.html           # Base layout (nav, modal shell)
│   ├── login.html            # Sign-in page
│   ├── index.html            # Dashboard page
│   ├── requests.html         # Full request list page
│   ├── request_detail_page.html
│   └── partials/             # HTMX-swappable fragments
│       ├── stats_cards.html
│       ├── request_list.html
│       ├── request_form.html
│       ├── request_detail.html
│       └── status_badge.html
├── static/
│   ├── style.css             # Custom styles (bento grid, badges)
│   ├── logo.png              # Project logo
│   └── favicon.png           # Browser favicon
├── data/                     # SQLite database (git-ignored)
├── go.mod
├── go.sum
├── LICENSE
└── README.md
```

---

## Dataset Request Fields

| Field | Description |
|---|---|
| Title | Short description of the dataset needed |
| Description | Physics process, energy range, selection criteria |
| Requester | Name and CERN email |
| Working Group | e.g. Tracker WG, Calorimetry WG |
| Dataset Type | Simulation (MC), Reconstruction, Raw Detector, MC Truth, Analysis Ntuple, Calibration |
| Use Case | Physics Analysis, Detector Design, Detector Simulation, ML Training/Evaluation, Benchmarking, Calibration |
| Format | ROOT, EDM4hep, HDF5, Parquet, … |
| Estimated Size | Number of events or file size |
| Due Date | When the data is needed |
| Priority | Low / Medium / High / Critical |
| Tags | Free-form labels (e.g. `fcc-hh`, `geant4`, `delphes`) |
| Notes | Generator settings, pile-up conditions, special requirements |

---

## Acknowledgements

Built with the assistance of [Claude](https://claude.ai) (Anthropic).
