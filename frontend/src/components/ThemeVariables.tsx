import React from 'react';

// Defines the CSS variable palette for both themes. Components that want
// to respect the theme toggle should use var(--bg-primary) etc instead of
// hardcoded hex colors. Not every component in the app has been migrated
// to these variables yet - this sets up the infrastructure and covers the
// pieces built after it was introduced; older components with hardcoded
// colors will need a follow-up pass to fully match.
export const ThemeVariables: React.FC = () => (
  <style>{`
    :root {
      --bg-primary: #1e1f22;
      --bg-secondary: #2b2d31;
      --bg-tertiary: #313338;
      --border-color: #3f4147;
      --text-primary: #f2f3f5;
      --text-secondary: #8a8f98;
      --accent: #5865f2;
      --accent-hover: #4752c4;
      --success: #23a55a;
      --warning: #f0b232;
      --danger: #da373c;
    }

    [data-theme="light"] {
      --bg-primary: #ffffff;
      --bg-secondary: #f2f3f5;
      --bg-tertiary: #e9eaed;
      --border-color: #d4d6da;
      --text-primary: #1e1f22;
      --text-secondary: #5c6066;
      --accent: #5865f2;
      --accent-hover: #4752c4;
      --success: #23a55a;
      --warning: #c57b0f;
      --danger: #da373c;
    }

    body {
      background: var(--bg-primary);
      color: var(--text-primary);
    }
  `}</style>
);