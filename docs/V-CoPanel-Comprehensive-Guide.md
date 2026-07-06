<div align="center">

# V-CoPanel Windows — Comprehensive Technical Guide

**Version 2.0.1 · VJECTS Architecture Team**

*The complete reference for architecture, internals, operations, and contribution.*

</div>

---

## Table of Contents

1. [Introduction & Vision](#1-introduction--vision)
2. [The Origin Story](#2-the-origin-story)
3. [Architecture Overview](#3-architecture-overview)
4. [Directory Structure](#4-directory-structure)
5. [Supported Stack & Services](#5-supported-stack--services)
6. [The Offline-First Philosophy](#6-the-offline-first-philosophy)
7. [Project Lifecycle: Provision & Eject](#7-project-lifecycle-provision--eject)
8. [Runtime Isolation & Shims Engine](#8-runtime-isolation--shims-engine)
9. [Shared Services & Service Manager](#9-shared-services--service-manager)
10. [Security & Trust Model](#10-security--trust-model)
11. [System Reset & Recovery Engine](#11-system-reset--recovery-engine)
12. [License & Ownership](#12-license--ownership)

---

## 1. Introduction & Vision

**V-CoPanel** is not another XAMPP clone or a Docker wrapper. It is a purpose-built, production-grade local development environment for Windows, designed to eliminate every single friction point that makes Windows development painful.

The central engine is written in **Go (Golang)** — a language chosen deliberately for its:
- Sub-millisecond startup time and negligible memory footprint (~5MB RAM)
- Native Windows process control without shelling out to PowerShell for routine operations
- Rock-solid concurrency for managing multiple simultaneous background services

The primary design mandate was a single, uncompromising constraint: **zero global system modifications.** This means:
- No entries in the Windows Registry
- No modifications to system `PATH` or environment variables
- No `C:\Program Files` installations
- No UAC prompts beyond initial execution

The result is a completely portable engine. Move the folder to a USB drive. Run it on a different machine. Everything works identically.

---

## 2. The Origin Story

### The Problem

I am a die-hard Linux developer. On Linux, the operating system cooperates with you. Processes behave. Permissions make sense. Multi-service orchestration is straightforward. Then a specific project forced me onto Windows, and what followed was a systematic dismantling of every assumption I had about rational software behavior.

PHP versions collided in system environment variables. Port conflicts materialized from nowhere. Background queue workers crashed silently with zero indication of failure. The `PATH` variable became a war zone.

### The Alternatives That Failed Me

I consulted AI assistants. They were enthusiastic. The suggestions were useless.

**Docker + WSL2** turned out to be the most absurd of all. I found myself writing 80-character terminal commands simply to execute `php artisan migrate`. Docker consumed 16 gigabytes of RAM to keep three completely empty containers alive. The WSL2 filesystem sync introduced enough latency to make hot-reload development actively painful.

**Laravel Herd** presented a polished UI. Then I clicked on anything useful — multi-PHP version management, proper queue runners, advanced project controls — and encountered a paywall demanding substantial annual subscription fees for functionality that should be baseline.

**Laragon** was a time capsule. Its UI design appeared frozen somewhere around the Windows XP era. While functional for basic use cases, it provided no path to the isolated, multi-version, enterprise-capable workflow I required.

### The Solution

After enough frustration, I stopped looking for a tool and built one. **V-CoPanel** was engineered from scratch in Go, with one rule: it had to work the way a Linux developer expects things to work, but natively on Windows.

> *"I didn't build V-CoPanel because I love Windows. From me — a Linux user — to the entire Windows developer community: **YOU'RE WELCOME.**"*
> — Founder, VJECTS

---

## 3. Architecture Overview

V-CoPanel operates as a single compiled binary (`bridge.exe`) that acts as a **local orchestration engine**. Its responsibilities are divided into distinct, decoupled subsystems:

```
┌─────────────────────────────────────────────────────────┐
│                    bridge.exe (Go Binary)                │
│                                                         │
│  ┌─────────────┐  ┌──────────┐  ┌────────────────────┐ │
│  │  HTTP API   │  │  Router  │  │  Static File Server │ │
│  │  /api/v1/*  │  │ (Mux)    │  │  /web/ (Dashboard) │ │
│  └──────┬──────┘  └────┬─────┘  └────────────────────┘ │
│         │              │                                 │
│  ┌──────▼──────────────▼─────────────────────────────┐  │
│  │              Internal Subsystems                   │  │
│  │                                                   │  │
│  │  precheck    │ Boot engine, asset extraction       │  │
│  │  database    │ MariaDB orchestration               │  │
│  │  mesh        │ Internal reverse proxy              │  │
│  │  projectmgr  │ Project CRUD & state management     │  │
│  │  shims       │ Runtime wrapper generation          │  │
│  │  envmanager  │ .env sandbox injection              │  │
│  └───────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────┘
         │
         ▼
┌─────────────────────────────────────────────────────────┐
│                  workspace/ (Runtime Domain)             │
│  core/           Minimal PHP for internal tooling       │
│  runtimes/       Isolated PHP, Node, Go installations   │
│  shared-services/  MariaDB, Redis, Mailpit, phpMyAdmin  │
│  projects/       Default project directory              │
└─────────────────────────────────────────────────────────┘
```

---

## 4. Directory Structure

```
V-CoPanel Windows/
│
├── bridge.exe                   ← Compiled Go engine (excluded from source repo)
├── start.bat                    ← Primary launcher
├── LICENSE                      ← V-CoPanel Public & Commercialization License
├── README.md
│
├── internal/                    ← All Go source code
│   ├── api/
│   │   ├── handlers_projects.go ← Project CRUD, provision, eject APIs
│   │   ├── handlers_services.go ← Service start/stop/status APIs
│   │   ├── handlers_system.go   ← System info, engine reset APIs
│   │   └── router.go            ← HTTP route registration
│   ├── database/
│   │   ├── db.go                ← System DB (SQLite/internal state)
│   │   └── mariadb.go           ← MariaDB process & DB orchestration
│   ├── mesh/
│   │   └── proxy.go             ← Internal reverse proxy for .vcopanel.internal
│   ├── precheck/
│   │   └── precheck.go          ← Asset extraction, port checks, shortcut creation
│   ├── projectmanager/
│   │   └── manager.go           ← Project state machine
│   ├── shims/
│   │   └── shims.go             ← php.bat, npm.cmd, go.bat generation
│   ├── envmanager/
│   │   └── env.go               ← .env sandbox block inject/strip
│   ├── logo.png                 ← Application logo
│   └── icon.ico                 ← Application icon (for desktop shortcut)
│
├── web/                         ← Frontend dashboard (no framework)
│   ├── views/                   ← Full HTML page templates
│   ├── components/              ← Reusable HTML partials (sidebar, header)
│   └── css/
│       └── style.css            ← Complete UI stylesheet
│
├── pc-assets/                   ← Offline runtime vault (excluded from Git)
│   ├── php/                     ← PHP 8.x Windows 64-bit zip archives
│   ├── node/                    ← Node.js Windows 64-bit zip archives
│   ├── mariadb/                 ← MariaDB Windows 64-bit zip archive
│   ├── redis/                   ← Redis Windows 64-bit zip archive
│   ├── mailpit/                 ← Mailpit Windows zip archive
│   ├── phpmyadmin/              ← phpMyAdmin zip archive
│   ├── composer/                ← composer.phar
│   ├── vcredist/                ← VC++ Redistributable installer
│   └── README.md                ← Developer guide for populating this directory
│
├── docs/
│   ├── Screenshots/             ← UI screenshots (00.png – 12.png)
│   └── V-CoPanel-Comprehensive-Guide.md
│
└── workspace/                   ← Auto-generated runtime domain (excluded from Git)
    ├── core/                    ← Minimal PHP for internal system tooling
    ├── runtimes/                ← Extracted runtime installations
    ├── shared-services/         ← Persistent service data (MariaDB files, etc.)
    └── projects/                ← Default project root directory
```

---

## 5. Supported Stack & Services

### Isolated Per-Project Runtimes

| Runtime | Supported Versions | Notes |
|---------|--------------------|-------|
| **PHP** | 8.2, 8.3, 8.4, 8.5 | Thread-safe Windows builds. Composer included. |
| **Node.js** | v20 LTS, v22 LTS | NPM, Vite, Webpack, Next.js all supported. |
| **Go** | 1.23+ | Full Go toolchain available per project. |

### Persistent Shared Services

| Service | Port(s) | Description |
|---------|---------|-------------|
| **MariaDB 11.4** | `3306` | MySQL-compatible enterprise relational DB |
| **Redis 5.0** | `6379` | In-memory cache for sessions, queues, caching |
| **phpMyAdmin 5.2** | `8881` | Web-based DB management UI |
| **Mailpit** | SMTP `3025` · UI `8025` | Local email catcher & webmail interface |

---

## 6. The Offline-First Philosophy

A frequently asked question: why ship runtime `.zip` archives instead of downloading them on first run?

The answer is absolute reliability over convenience.

**Air-Gapped & Firewall Environments.** Many enterprise developers operate behind strict corporate network policies that block outbound downloads. V-CoPanel boots completely and immediately in any environment, including fully isolated intranet machines.

**Protection Against Link Rot.** Download URLs change. APIs get deprecated. Build server artifacts get removed. If V-CoPanel depended on fetching runtimes from vendor servers, a single URL change would silently break every fresh installation worldwide. Offline archives guarantee that the software runs with identical reliability in 2026, 2031, and 2036.

**Guaranteed Version Consistency.** When teams download runtimes dynamically, minor patch-level version differences (PHP 8.3.10 vs 8.3.12) can introduce subtle, difficult-to-reproduce bugs. With pre-packaged archives, every member of a team runs on the exact same byte-for-byte runtime.

**Zero-Day Onboarding Speed.** Extracting local zip files at SSD speeds completes in under 5 seconds. Downloading hundreds of megabytes of runtimes on first launch is a terrible user experience, particularly on slow or metered connections.

---

## 7. Project Lifecycle: Provision & Eject

### Provision — Connecting a Project

When you add an existing project directory to V-CoPanel, the engine executes a Provision sequence:

1. **Framework Detection.** The engine scans the project root for known markers (`artisan` for Laravel, `package.json` for Node, etc.) and auto-classifies the project type.
2. **Database Generation.** Connects to the embedded MariaDB instance and creates a dedicated database with a unique identifier (e.g., `db_a1b2c3d4`).
3. **Port Assignment.** Assigns a unique HTTP port from the sequential pool starting at `8882`.
4. **Shims Injection.** Generates runtime-specific wrapper scripts in the project root.
5. **Sandbox Block Injection.** Appends a protected credential block to the project's `.env` file.

The project is now registered in V-CoPanel's internal state database and appears in the dashboard with live status.

### Eject — Removing a Project Without a Trace

V-CoPanel has zero tolerance for vendor lock-in. The Eject operation:

1. Drops the project's dedicated MariaDB database.
2. Removes the Sandbox Block from `.env`, leaving all other variables untouched.
3. Deletes all shim wrapper files (`php.bat`, `npm.cmd`, `go.bat`, `composer.bat`) from the project root.
4. Unregisters the project from the internal state database.
5. Releases the assigned port back to the available pool.

**Your source code is returned to its exact original state.** No hidden files remain. No registry entries. No orphaned processes.

---

## 8. Runtime Isolation & Shims Engine

The Shims Engine is the core mechanism that makes true per-project runtime isolation possible without modifying the system `PATH`.

### How It Works

When a project is provisioned for PHP 8.4 + Node v22, the engine writes wrapper scripts into the project root:

```batch
:: php.bat (example content)
@echo off
"C:\path\to\V-CoPanel\workspace\runtimes\php-8.4\php.exe" %*
```

Now, when a developer opens a terminal in that project directory and types:
```
php artisan serve
```

Windows resolves `php` to the local `php.bat` shim before searching the system `PATH`. The command transparently routes through V-CoPanel's isolated PHP 8.4 installation.

Switch to a different project configured for PHP 8.2, and the same `php` command routes to the PHP 8.2 runtime — with zero terminal configuration, no `phpenv`, no `nvm`, no profile sourcing.

### Sandbox Block (.env Injection)

Upon provisioning, the `envmanager` appends a protected block to the project's `.env`:

```
# --- BEGIN V-COPANEL SANDBOX BLOCK --- DO NOT EDIT ---
DB_CONNECTION=mysql
DB_HOST=127.0.0.1
DB_PORT=3306
DB_DATABASE=db_a1b2c3d4
DB_USERNAME=root
DB_PASSWORD=
REDIS_HOST=127.0.0.1
REDIS_PORT=6379
MAIL_MAILER=smtp
MAIL_HOST=127.0.0.1
MAIL_PORT=3025
# --- END V-COPANEL SANDBOX BLOCK ---
```

On Eject, this block is stripped precisely, leaving all surrounding content intact.

---

## 9. Shared Services & Service Manager

Unlike per-project runtimes, the core data services run as persistent background processes that serve all projects simultaneously:

**MariaDB** stores its data files in `workspace/shared-services/mariadb/data/`. This means your databases survive complete V-CoPanel uninstallations and re-installations — move the folder, and your data moves with it.

**Redis** runs in-memory and is restarted cleanly on each engine boot.

**Mailpit** provides a local SMTP server on port `3025` and a web UI on port `8025`. All outbound email from any provisioned project is intercepted here, preventing accidental emails to real addresses during development.

**phpMyAdmin** runs as a PHP application served through V-CoPanel's internal minimal PHP runtime. It connects exclusively to the embedded MariaDB instance.

The Service Manager tab in the dashboard provides live status, port visibility, and direct launch links for phpMyAdmin and Mailpit.

---

## 10. Security & Trust Model

### On the Pre-Packaged Archives

The `pc-assets/` directory contains pre-packaged `.zip` archives of all runtimes. These are unmodified builds sourced directly from official vendor distributions.

For developers who require verified archives: download the exact version directly from the official vendor sites (`php.net`, `nodejs.org`, `mariadb.org`, `redis.io`) and replace the corresponding archive in `pc-assets/` using the exact filename specified in `pc-assets/README.md`. V-CoPanel will use your verified copy without modification.

### Network Behavior

V-CoPanel's bridge engine makes **zero outbound network requests** during normal operation. The only network activity is:
- Listening on `localhost:8880` for the dashboard
- Listening on service ports (`3306`, `6379`, `8881`, `8025`, `3025`)
- Serving the project's own application on its assigned port

There is no telemetry, no analytics, no update checks, and no cloud dependency of any kind.

---

## 11. System Reset & Recovery Engine

V-CoPanel includes a full factory reset capability accessible from the Engine dashboard.

The reset sequence executes entirely within the Go engine:

1. Signals all managed processes to terminate.
2. Waits for OS file handle release (Windows requires a grace period before files can be deleted).
3. Scans all files within the `workspace/` directory and sorts paths by length (deepest paths first) to ensure correct deletion order.
4. Deletes files with a three-tier retry loop — essential for handling Windows file lock scenarios.
5. Streams real-time progress (files deleted / total files, percentage) to the dashboard UI.
6. On completion, generates a minimal recovery batch script and launches it detached via PowerShell.
7. The recovery script terminates the engine, removes the compiled binary, and restarts the platform launcher (`start.bat`), which triggers a fresh asset extraction on next boot.

The entire sequence runs without user intervention and completes with a smooth browser redirect to the boot screen.

---

## 12. License & Ownership

<div align="center">

Engineered and designed by the **VJECTS Architecture Team**.

This is a fully self-funded, independent project — built with zero corporate
backing, zero sponsorship, and an unreasonable amount of personal determination.

**Licensed under the [V-CoPanel Public & Commercialization License v2.0.1](../LICENSE)**

| | |
|---|---|
| 🌐 **Official Website** | [vjects.com](https://vjects.com) |
| 💬 **Direct Support** | [Telegram @vjects](https://t.me/vjects) |
| 📦 **Repository** | [github.com/vjects/V-CoPanel-Windows](https://github.com/vjects/V-CoPanel-Windows) |
| 🐛 **Bug Reports** | [GitHub Issues](https://github.com/vjects/V-CoPanel-Windows/issues) |

*Copyright © 2026 VJECTS. All rights reserved.*
*The one and only authoritative domain for this project and VJECTS is `vjects.com`.*

</div>
