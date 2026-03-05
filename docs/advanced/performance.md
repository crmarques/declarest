# Performance and scale

This page covers practical scaling guidance for larger DeclaREST usage.

## Scale dimensions

Performance usually depends on:

- number of resources per sync scope
- API latency and rate limits
- metadata complexity (transforms/selectors)
- reconcile frequency

## Practical tuning points

- Keep `SyncPolicy.source.path` scopes focused; avoid one giant policy.
- Use reasonable `syncInterval` values for API capacity.
- Configure `ManagedServer.http.requestThrottling` for bounded concurrency.
- Use incremental-friendly repository change patterns (small, scoped commits).

## Repository and policy design

- Split large domains into multiple non-overlapping `SyncPolicy` paths.
- Avoid frequent churn in identity fields and logical path structures.
- Keep metadata minimal and targeted to required overrides.

## API pressure control

- Start with conservative concurrency and queue settings.
- Measure failed/queued requests before increasing throughput.
- Use full-resync cadence carefully; frequent full syncs increase load.

## Observability signals to watch

- sync duration trends
- reconcile error rates
- `resourceStats.failed` and repeated retries
- managed API latency and throttling responses

## Capacity strategy

1. Baseline with one narrow scope.
2. Increase scope count gradually.
3. Add policy partitioning before raising concurrency aggressively.
4. Re-validate after metadata or API contract changes.
