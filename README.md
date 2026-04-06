# Snip

A self-hosted URL shortening service with built-in click analytics.

## Overview

Snip is a web application with a backend API and persistent storage, built as a Replicated Bootcamp project.

## Features

### Link Creation
Users paste a long URL and get a short slug back (auto-generated or custom). Optional metadata: a label/description and an expiration date.

### Analytics Dashboard
Per-link stats showing total clicks, clicks over time (chart), and top referrers. A home screen shows all links ranked by click volume.

### Click Redirect
Hitting a short link (e.g. `snip.example.com/abc123`) looks up the slug, records the timestamp and referrer, and issues an HTTP redirect to the destination URL.

## Technical Requirements

- **PostgreSQL** — persistent storage for links and click events
- **Redis** — read-through cache on the redirect path for low-latency redirects
- **`/healthz` endpoint** — checks both database and cache connections; returns a structured JSON response
- **No authentication** — single-user/open dashboard

## Replicated Bootcamp

This project is packaged and distributed using the [Replicated](https://replicated.com) platform. See [`rubric.md`](./rubric.md) for acceptance criteria across all tiers.
