import { PostHog } from 'posthog-node';
import type { LoadContext, Plugin } from '@docusaurus/types';

export default function posthogPlugin(_context: LoadContext): Plugin {
  return {
    name: 'posthog-analytics',

    async postBuild({ outDir, routes }) {
      const apiKey = process.env.POSTHOG_API_KEY;
      const host = process.env.POSTHOG_HOST;

      if (!apiKey) {
        return;
      }

      const client = new PostHog(apiKey, {
        ...(host ? { host } : {}),
        flushAt: 1,
        flushInterval: 0,
        enableExceptionAutocapture: true,
      });

      try {
        client.capture({
          distinctId: 'raid-docsite-build',
          event: 'docs built',
          properties: {
            page_count: routes.length,
            out_dir: outDir,
            environment: process.env.NODE_ENV ?? 'production',
          },
        });
      } catch (err) {
        client.captureException(err, 'raid-docsite-build');
      } finally {
        await client.shutdown();
      }
    },
  };
}
