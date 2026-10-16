# Relay port reservation

Approved scope: one cooperating relay per user account and port; standalone and
embedded hosting share the reservation. Preserve normal Go socket reuse and fast
restart. Explicit IPv4/IPv6 wildcard addresses select only that IP family.

Implementation proceeds inline in the existing unborn repository.

- Add failing tests for reservations across addresses and processes, release after
  close/crash/bind failure, immediate restart, and explicit wildcard families.
- Add an OS-held lock in a stable account-home directory, independent of application
  configuration and environment-selected cache directories. Never unlink lock files.
  Use flock on supported Unix systems and LockFileEx on Windows. Release only after
  listeners, connections and workers close. Unsupported systems fail explicitly.
- Acquire before fixed-port binds. For internal port-zero tests, acquire the actual
  port after the first socket is assigned; failure closes all acquired resources.
- Preserve module transaction ordering and rollback; update documentation.
- Run race tests, vet, build matrix and builds. Runtime networking verification is
  limited to this macOS host; Windows/Linux cross-builds are not runtime evidence.

The lock is cooperative, per account, not protection against unrelated software,
older binaries or other users. Different specific addresses still need different
ports for cooperating relay instances. No production process is restarted here.

Implemented and verified locally on macOS. Initial regression tests failed for
address overlap, cross-process overlap and IPv4 wildcard family; all now pass.
Race tests cover concurrent reservations, failed bind cleanup, process kill, close
and immediate restart after HTTP traffic. Built-in relay transaction tests pass.
Independent review identified os/user.Current's environment fallback for unknown
UIDs; explicit LookupId now requires a system account record and fails closed.
Child-process test uses a different HOME/cache path to verify shared reservations.
Linux and Windows compile successfully; runtime behavior there remains unverified.

The complete matrix exposed concurrent Root.OpenFile(O_CREATE) returning ENOENT
on macOS for a new inode. A minimal standalone reproducer isolated this from
networking. Atomic O_CREATE|O_EXCL creation followed by opening on EEXIST removed
the failure; no lock file is removed or replaced.

Final verification completed successfully: 100 focused race repetitions; root
race tests and vet; make check; make check-modules; make build-core and make build;
Windows and Linux CLI cross-compilation. An isolated binary smoke check rejected
a second address family on the same reserved port and restarted after process kill.
No running user instance was restarted and no application configuration changed.
