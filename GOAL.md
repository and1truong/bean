# Goal: make the shared bar chart readable

Status: complete

Fix the cramped chart shown in SaaS: separate linked rows, give the chart a visible title and matching card surface, and retain proportional bars and drill navigation. Keep all business behavior in metadata.

## Verification

- Reproduce touching linked chart rows in a browser regression.
- Verify row spacing, drill behavior, signed values and light/dark/mobile screenshots.
- Finish with make check and make build; reload only the SaaS demo on port 8084.

## Evidence

Browser regression reproduced zero spacing before the fix and passes with at least 8px between rows. All three SaaS journeys pass, including chart drill; light/dark/mobile screenshots reviewed. Existing signed chart tests pass. Final make check passes 90 frontend tests and 23 browser journeys; make build passes. The SaaS server on port 8084 now serves the updated shared renderer.
