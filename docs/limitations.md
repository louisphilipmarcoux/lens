# Known Limitations and Future Work

This document honestly describes what Lens does not handle and what would need to change to support each item.

## Multi-Tenant Billing and Access Control

**Current state:** Lens is single-tenant. All data is visible to all users.

**What would change:** Add an authentication layer (OAuth2/OIDC), a tenant ID column to all ClickHouse tables, query-time tenant filtering, and a billing service that meters ingestion volume per tenant. Estimated complexity: 4-6 weeks.

## Global Multi-Region Deployment

**Current state:** Lens runs in a single region. All components are co-located.

**What would change:** Add cross-region ClickHouse replication (or use ClickHouse Cloud), deploy ingestion backends per region with a global query federation layer. The agent already ships to a configurable URL, so multi-region agents work today. Estimated complexity: 6-8 weeks.

## Mobile App

**Current state:** Dashboard is web-only (React).

**What would change:** Build a React Native app or use a mobile-responsive web design. The query API is already HTTP/gRPC, so the backend needs no changes. Estimated complexity: 4-6 weeks for a basic mobile dashboard.

## SLA / Error Budget Tracking

**Current state:** Lens has alerting but no formal SLI/SLO/error budget system.

**What would change:** Add SLO configuration (target percentage, time window), a burn-rate calculator, and error budget remaining display on dashboards. The underlying metric data already exists. Estimated complexity: 2-3 weeks.

## Synthetic Monitoring

**Current state:** Lens monitors real traffic only. No synthetic probes.

**What would change:** Add a probe scheduler service that sends HTTP/gRPC/TCP checks at intervals, records results as metrics, and feeds them into the alerting pipeline. Estimated complexity: 3-4 weeks.

## Custom Plugin System

**Current state:** Collectors, parsers, and alerting integrations are compiled into the binary.

**What would change:** Define a plugin interface (likely using Go plugins or a subprocess/gRPC model), add a plugin registry, and support dynamic loading. This is architecturally significant — the gRPC subprocess model is more maintainable than Go's native plugin system. Estimated complexity: 4-6 weeks.
