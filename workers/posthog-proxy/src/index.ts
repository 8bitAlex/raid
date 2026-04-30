const POSTHOG_ASSETS_HOST = 'https://us-assets.i.posthog.com';
const POSTHOG_API_HOST = 'https://us.i.posthog.com';

export default {
  async fetch(request: Request): Promise<Response> {
    const url = new URL(request.url);

    // Strip the /ingest prefix
    const path = url.pathname.replace(/^\/ingest/, '') || '/';

    // Route static assets (JS bundle) to the CDN host, everything else to the API host
    const origin = path.startsWith('/static/') ? POSTHOG_ASSETS_HOST : POSTHOG_API_HOST;
    const target = new URL(path + url.search, origin);

    const proxied = new Request(target.toString(), {
      method: request.method,
      headers: request.headers,
      body: request.method !== 'GET' && request.method !== 'HEAD' ? request.body : null,
    });

    const response = await fetch(proxied);

    // Pass through with CORS headers so the browser accepts the response
    const headers = new Headers(response.headers);
    headers.set('Access-Control-Allow-Origin', request.headers.get('Origin') ?? '*');
    headers.set('Access-Control-Allow-Credentials', 'true');

    return new Response(response.body, {
      status: response.status,
      headers,
    });
  },
};
