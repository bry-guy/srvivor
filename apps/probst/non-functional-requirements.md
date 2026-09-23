# Non-functional requirements

- HTTP client only; never connect to a database.
- Inject the bearer credential through the environment, not a command-line argument.
- Require HTTPS except for loopback tests; never forward credentials through redirects.
- Bound requests to ten seconds and response decoding to four MiB.
- Delegate authorization to service-authenticated API transactions; never claim human login.
- Validate mutations using disposable PostgreSQL, preserving historical gameplay data.
