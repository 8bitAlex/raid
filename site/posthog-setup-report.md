# PostHog post-wizard report

The wizard has completed a PostHog integration for the Raid documentation site. A custom Docusaurus plugin (`plugins/posthog.ts`) was created that uses the `posthog-node` SDK to capture a `docs built` event each time the site is built. The plugin hooks into the Docusaurus `postBuild` lifecycle, which runs in Node.js after every successful build. It is registered in `docusaurus.config.ts`. Environment variables (`POSTHOG_API_KEY` and `POSTHOG_HOST`) are read from `.env` and are never hardcoded. The plugin exits silently when `POSTHOG_API_KEY` is unset, so local builds without the key are unaffected. All events are flushed synchronously before the build process exits (`flushAt: 1`, `flushInterval: 0`, `shutdown()`). Exception autocapture and manual `captureException` are both enabled for error tracking.

| Event | Description | File |
|-------|-------------|------|
| `docs built` | Fired after every successful documentation build. Includes `page_count` (number of routes generated), `out_dir` (output directory path), and `environment` (Node `NODE_ENV`). | `plugins/posthog.ts` |

## Next steps

We've built some insights and a dashboard for you to keep an eye on build activity:

- **Dashboard — Analytics basics**: https://us.posthog.com/project/403603/dashboard/1527369
- **Docs builds over time** (daily trend, last 30 days): https://us.posthog.com/project/403603/insights/YZHNQIhz
- **Total docs builds** (90-day total): https://us.posthog.com/project/403603/insights/nkm9xGi7
- **Average pages per build** (daily avg page_count, last 30 days): https://us.posthog.com/project/403603/insights/RoHTskgj

### Agent skill

We've left an agent skill folder in your project. You can use this context for further agent development when using Claude Code. This will help ensure the model provides the most up-to-date approaches for integrating PostHog.
