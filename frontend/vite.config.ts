import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'
import path from 'path'

const kib = 1024

export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: {
      // Force all React imports to use the same instance (fixes tldraw duplicate React).
      react: path.resolve(__dirname, 'node_modules/react'),
      'react-dom': path.resolve(__dirname, 'node_modules/react-dom'),
    },
  },
  build: {
    cssCodeSplit: true,
    rolldownOptions: {
      output: {
        codeSplitting: {
          includeDependenciesRecursively: false,
          // The editor is an async route. Keep its dependency graph in stable,
          // cacheable chunks without pulling those modules into the app shell.
          groups: [
            {
              name: 'editor-monaco',
              test: /node_modules[\\/](?:monaco-editor|@monaco-editor)[\\/]/,
              priority: 80,
              maxSize: 600 * kib,
            },
            {
              name: 'editor-tldraw-ui',
              test: /node_modules[\\/]@tldraw[\\/]tldraw[\\/]/,
              priority: 70,
            },
            {
              name: 'editor-tldraw-ui-components',
              test: /node_modules[\\/]tldraw[\\/]dist-esm[\\/]lib[\\/]ui[\\/]/,
              priority: 75,
            },
            {
              name: 'editor-tldraw-shapes',
              test: /node_modules[\\/]tldraw[\\/]dist-esm[\\/]lib[\\/]shapes[\\/]/,
              priority: 75,
            },
            {
              name: 'editor-tldraw-tools',
              test: /node_modules[\\/]tldraw[\\/]dist-esm[\\/]lib[\\/]tools[\\/]/,
              priority: 75,
            },
            {
              name: 'editor-tldraw-runtime',
              test: /node_modules[\\/]tldraw[\\/]/,
              priority: 70,
            },
            {
              name: 'editor-tldraw-core',
              test: /node_modules[\\/]@tldraw[\\/]editor[\\/]/,
              priority: 70,
            },
            {
              name: 'editor-tldraw-model',
              test: /node_modules[\\/]@tldraw[\\/]/,
              priority: 65,
            },
            {
              name: 'editor-richtext',
              test: /node_modules[\\/](?:@tiptap|prosemirror-|orderedmap|rope-sequence|w3c-keyname)[\\/]/,
              priority: 60,
            },
            {
              name: 'editor-components',
              test: /node_modules[\\/](?:@radix-ui|radix-ui|@floating-ui|react-remove-scroll|react-remove-scroll-bar|react-style-singleton|use-callback-ref|use-sidecar|aria-hidden|get-nonce)[\\/]/,
              priority: 60,
            },
            {
              name: 'editor-yjs',
              test: /node_modules[\\/](?:yjs|y-websocket|lib0)[\\/]/,
              priority: 60,
            },
            {
              name: 'vendor-react',
              test: /node_modules[\\/](?:react|react-dom|react-router|react-router-dom|scheduler)[\\/]/,
              priority: 50,
            },
            {
              name: 'vendor-animation',
              test: /node_modules[\\/]gsap[\\/]/,
              priority: 40,
            },
            {
              name: 'vendor-icons',
              test: /node_modules[\\/]lucide-react[\\/]/,
              priority: 40,
            },
            {
              name: 'vendor-shared',
              test: /node_modules[\\/]/,
              priority: 10,
              minShareCount: 2,
              minSize: 20 * kib,
            },
          ],
        },
      },
    },
  },
  server: {
    port: 5173,
    proxy: {
      '/api': 'http://localhost:8080',
      '/ws': {
        target: 'ws://localhost:8081',
        ws: true,
      },
    },
  },
})
