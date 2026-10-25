# UX audit corrections

The user approved all ten findings from the browser audit of dev-a0bead5.
Improve discoverability and first use while retaining the existing visual style,
native modules, API contracts, polling stability, secrets lifecycle and security.

## Approach

Use the existing shared UI components. A replacement navigation or a new
frontend framework would add scope without addressing the observed problems.
Changes are presentation and interaction fixes, not new backend capabilities.

## Required behavior

1. First-time E2E secret creation is a visible text action. Reveal/copy remain
   available for existing secrets. Instructions wrap outside the input.
2. Header connection settings open the owning module's actual configuration.
3. Relay distinguishes Not configured, Disabled, Connecting, Connected and
   Connection failed. Its empty overview explains the next step with a CTA.
4. The location chooser explains Local Ports and SSH, and exposes Remote HTTP
   and relay discovery as secondary routes. Unavailable modules retain reasons.
5. Location forms open over the original page. Back reopens the chooser and
   retains entered values; Cancel/Escape leave navigation unchanged. A successful
   save may navigate to the selected source. Standalone module forms stay intact.
6. Local listener and SSH host lists have case-insensitive search, clear empty
   results, explicit selection, and retain manual input on discovery failure.
7. Use plain field labels and explain technical terms where needed. Preserve
   canonical module names and English product text.
8. Main module creation actions have text. Small icon actions get 44px touch
   targets on narrow screens and coarse pointers.
9. Empty dashboard explains the localhost address result. Runtime remains
   available with its five existing fields, collapsed by default on narrow screens.
10. Long service forms keep submit/cancel accessible with a sticky action footer;
    instructions wrap at 320px. Preserve keyboard focus, Escape and polling drafts.

## Verification

Behavior tests cover state distinctions, header destination, chooser navigation,
back/draft lifecycle, filtered discovery and selection. Run frontend and Go checks
inside sbx. Verify real browser screens at desktop, 390px, 320px and 1024x600.
Use temporary configuration and fixtures for mutations, never the user's services.
