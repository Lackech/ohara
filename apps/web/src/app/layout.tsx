import type { Metadata } from 'next';
import { RootProvider } from 'fumadocs-ui/provider';
import 'fumadocs-ui/style.css';
import './globals.css';

export const metadata: Metadata = {
  title: 'Ohara — Agent-Optimized Documentation',
  description: 'Documentation platform built for AI agents and human developers, powered by Diataxis.',
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en" suppressHydrationWarning>
      <body>
        <RootProvider>
          {children}
        </RootProvider>
      </body>
    </html>
  );
}
