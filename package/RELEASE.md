# aux4/curl 1.3.2

## Added

- **`--brokerToken` for broker login.** `curl oauth login --broker <url>` now
  attaches `Authorization: Bearer <brokerToken>` to the broker's authorize-url,
  session-poll and exchange calls when a token is supplied, so a secured broker
  can authenticate the caller. curl is pure mechanism here: it does not decide
  where the token comes from — the calling package supplies it (e.g. an aux4
  identity token for an aux4 broker). An empty `--brokerToken` sends no header.

## Changed

- A `401`/`403` from the broker's authorize-url now yields a clear message
  ("requires authentication … no valid broker token was supplied. Pass
  --brokerToken …") instead of a generic HTTP error.
