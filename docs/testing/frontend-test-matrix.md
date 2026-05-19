# Frontend Test Matrix

| Surface | File | Critical Behaviors | Recommended Layer | Priority | Notes |
|---|---|---|---|---|---|
| Transactions page | `frontend/src/pages/Transactions.tsx` | URL search param persistence, filter/query mapping, mutation feedback, search pagination | Frontend Unit, Playwright | Highest | page with the densest interaction surface |
| App shell and navigation | `frontend/src/components/AppLayout.tsx`, `frontend/src/components/Sidebar.tsx`, `frontend/src/components/CommandPalette.tsx` | global navigation behavior, sidebar persistence, keyboard shortcuts, portal/floating navigation UI | Frontend Unit, Playwright | High | shared interaction surface across the whole app |
| Dashboard page | `frontend/src/pages/Dashboard.tsx` | summary/date state persistence, drill-down navigation, floating UI interactions, chart and heatmap state | Frontend Unit, Playwright | High | high-value page with multiple stateful views |
| Settings page | `frontend/src/pages/Settings.tsx` | tab persistence in URL, save feedback, settings value validation surfaces | Frontend Unit, Playwright | High | directly tied to repo URL-state rule |
| Muted page | `frontend/src/pages/MutedPage.tsx` | tab persistence, inline editing, filtering, confirm flows, mutation entry points | Frontend Unit, Playwright | High | non-trivial management surface with URL state and mutations |
| Rules page | `frontend/src/pages/Rules.tsx` | CRUD entry points, inline editing affordances, confirmation flow | Frontend Unit, Playwright | High | pairs with backend rules workstream |
| Setup wizard | `frontend/src/pages/setup/Wizard.tsx` | branching setup flow, reader-specific transitions, OAuth/manual callback exchange fallback, completion path | Frontend Unit, Playwright | High | browser flow candidate after unit harness; step files are supporting detail |
| InlineSelect | `frontend/src/components/InlineSelect.tsx` | keyboard nav, outside click, avoid redundant commit, fixed-position dropdown | Frontend Unit | High | reusable primitive with interaction risk |
| LabelCombobox | `frontend/src/components/LabelCombobox.tsx` | label filtering, create/select/remove flows, portal dropdown and notification trigger | Frontend Unit | High | transaction labeling risk |
| FilterCombobox | `frontend/src/components/FilterCombobox.tsx` | input filtering, clear action, keyboard selection, outside click | Frontend Unit | Medium | reusable but simpler than transactions page |
| DateRangePicker | `frontend/src/components/DateRangePicker.tsx` | range selection, clear/apply, portal behavior, date/time carry-over | Frontend Unit | High | complex stateful input |
| ConfirmModal | `frontend/src/components/ConfirmModal.tsx` | confirm/cancel callback wiring, escape handling, overlay close | Frontend Unit | Medium | modal interaction primitive |
| SlideNotification | `frontend/src/components/SlideNotification.tsx` | timeout action, manual actions, dismissal timing | Frontend Unit | Medium | timer-driven UI |
| Tooltip hook | `frontend/src/hooks/useTooltip.tsx` | hover visibility, portal rendering, placement behavior | Frontend Unit | Medium | shared primitive |
