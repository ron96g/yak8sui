# Avoid ownership preview stream starvation

Browsers limit concurrent HTTP/1.1 connections per origin. An expanded ownership
hierarchy can consume that limit with long-lived relationship streams while the
resource-list stream remains open behind the detail dialog. A newly selected YAML
preview can then remain queued indefinitely and display only its loading state.

Pause the resource-list stream while the Ownership tab is active and restore it
when ownership inspection closes or another detail tab is selected. For each
preview selection, complete the existing authorized UID-scoped detail read before
opening the dedicated detail replacement stream. The stream remains the authority
for subsequent committed replacements, observation loss, and revocation. Tenant,
center, selection, and session changes still cancel both phases by generation and
request cancellation.

This bounds connection demand without weakening authorization or replacing UID
identity with resource names. It also gives the user an immediate current snapshot
when a long-lived stream must wait briefly for a browser connection slot.
