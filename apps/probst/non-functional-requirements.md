# Non-functional requirements

- HTTP client only; never connect to a database.
- Load credentials from an owner-only local JSON config (`~/.config/probst/config.json`) or the environment, never from command-line arguments; reject insecure, nonregular, or malformed config files without echoing their contents.
- Require HTTPS except for loopback tests; never forward credentials through redirects.
- Bound requests to ten seconds and response decoding to four MiB.
- Delegate authorization to service-authenticated API transactions; never claim human login.
- Validate mutations using disposable PostgreSQL, preserving historical gameplay data.
