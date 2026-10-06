# Synthetic test data

Every fixture in this directory was authored for automated tests and contains fictitious identities, dates and counters. None is a driver profile or session log copied from a player or vendor distribution.

- `bbs-before.odr`: UTF-8 starting profile with 1,000 stops.
- `bbs-after.odr`: UTF-16LE with BOM; 1,015 stops, including three additional early stops, and deliberately non-perfect ratings.
- `bcs-synthetic-session.txt`: fictitious session used to exercise trip parsing and exact shift completion.

These field labels are the local compatibility formats expected by the implementation. The examples are testing inputs, not files to install into OMSI 2/BCS or use to alter a real account.
