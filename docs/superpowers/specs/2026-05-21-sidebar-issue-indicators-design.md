# Sidebar Issue Indicators Design

## Context

Expensor has two user-actionable failure modes that are currently discoverable only after visiting their pages:

- Gmail authorization can become invalid, blocking email scans until the user reauthorizes from setup.
- Email extraction failures appear in the diagnostics queue, but the user must visit diagnostics to know that open items exist.

The sidebar should surface both states in expanded and collapsed layouts without changing the existing navigation structure.

## Visual Design

The setup/onboarding navigation item keeps its existing icon. When Gmail scanning is blocked by authorization readiness, show a subtle amber dot:

- Expanded sidebar: dot at the trailing edge of the setup row.
- Collapsed sidebar: dot at the top-right of the setup icon tile.

The diagnostics navigation item shows the count of open diagnostics:

- Expanded sidebar: compact amber pill at the trailing edge of the row.
- Collapsed sidebar: compact amber badge positioned at the top-right of the icon tile.

Collapsed badges must not be clipped by the sidebar's scroll container. The number must be visually centered inside the badge.

## Data Flow

Use existing frontend queries:

- `useReaderStatus('gmail')` to detect whether Gmail is ready to scan.
- `useExtractionDiagnostics('open')` to count open diagnostics.

Derive sidebar state:

- `setupNeedsAttention` is true when the Gmail reader status query succeeds and `ready` is false for the OAuth reader.
- `openDiagnosticsCount` is the length of the open diagnostics array.

No backend schema or API changes are required for this version. If the diagnostics payload becomes too heavy later, a dedicated count/summary endpoint can replace the list query.

## Component Scope

Update `frontend/src/components/Sidebar.tsx` only for rendering and data consumption. Keep the current `NavLink` map, tooltip behavior, and existing icons. Add focused helper rendering inside the sidebar if needed.

Add tests in `frontend/src/components/Sidebar.test.tsx` that mock query hooks or seed query data through the test query client.

## Accessibility

Do not rely on the dot alone for collapsed users. Fold alert context into the collapsed tooltip label:

- Setup tooltip should mention authorization attention when the dot is present.
- Diagnostics tooltip should include the open count when the count is non-zero.

Expanded labels remain the standard navigation labels; the visual indicators are supplemental.

## Testing

Add component tests for:

- Setup keeps the existing onboarding icon and shows an amber attention dot when Gmail readiness is false.
- The setup dot appears in collapsed mode.
- Diagnostics shows the open count in expanded mode.
- Diagnostics shows the open count in collapsed mode with non-clipping and centering classes.

Run the narrow frontend component test first, then the frontend test target.
