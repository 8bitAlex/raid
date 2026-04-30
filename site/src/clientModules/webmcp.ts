// WebMCP: expose raid docs tools to AI agents via the browser
// https://webmachinelearning.github.io/webmcp/
let controller: AbortController | null = null;

export function onRouteDidUpdate(): void {
  if (typeof navigator === 'undefined' || !('modelContext' in navigator)) {
    return;
  }

  if (controller) {
    controller.abort();
  }
  controller = new AbortController();
  const { signal } = controller;
  const mc = (navigator as any).modelContext;

  mc.registerTool({
    name: 'navigate_to_docs',
    description: 'Navigate to a section of the raid documentation.',
    inputSchema: {
      type: 'object',
      properties: {
        section: {
          type: 'string',
          enum: ['overview', 'usage', 'features', 'examples', 'references', 'whats-new'],
          description: 'The documentation section to navigate to.',
        },
      },
      required: ['section'],
    },
    execute: async ({ section }: { section: string }) => {
      const paths: Record<string, string> = {
        'overview':   '/docs/overview',
        'usage':      '/docs/category/usage',
        'features':   '/docs/category/features',
        'examples':   '/docs/category/examples',
        'references': '/docs/category/references',
        'whats-new':  '/docs/whats-new',
      };
      window.location.href = paths[section] ?? '/docs/overview';
    },
    signal,
  });

  mc.registerTool({
    name: 'get_install_command',
    description: 'Returns the command to install raid on the current platform.',
    inputSchema: {
      type: 'object',
      properties: {
        platform: {
          type: 'string',
          enum: ['macos', 'linux', 'windows'],
          description: 'Target platform.',
        },
      },
      required: ['platform'],
    },
    execute: async ({ platform }: { platform: string }) => {
      const commands: Record<string, string> = {
        macos:   'brew install 8bitalex/tap/raid',
        linux:   'curl -sSL https://github.com/8bitalex/raid/releases/latest/download/raid_linux_amd64.tar.gz | tar -xz && sudo mv raid /usr/local/bin/',
        windows: 'scoop bucket add 8bitalex https://github.com/8bitalex/scoop-bucket && scoop install raid',
      };
      return commands[platform] ?? commands.macos;
    },
    signal,
  });
}
