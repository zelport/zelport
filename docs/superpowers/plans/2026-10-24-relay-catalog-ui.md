# Relay catalog and dashboard warning repair

User scope: replace Add another domain with a plus outside service rows, show
ports in the shared catalog, and use a warning triangle rather than inline
connection-warning prose on the Dashboard.

Implementation: one device-header plus, service chooser for multiple services,
free-domain suggestion on repeated imports, provider-port address labels and
Imported / Not imported state. Dashboard uses shared warningIndicator.

Baseline: /tmp/zelport-before-catalog-ui.tgz. Tests failed on missing ports,
missing header action and missing warning indicator before implementation.
make check passed (209 frontend tests, Go race tests, vet, 39+3 packaging tests).
Two additional single/multiple-service import tests passed (47 relay UI tests).
Browser: actual MacBook service imported on 7070; plus opens speedtest-2 domain
without saving; HTTP warning triangle/tooltip verified on isolated 7274.
MacBook's own Dashboard requires the updated binary on that remote device.
