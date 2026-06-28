# Third Party Notices

This module recreates fund research workflows inspired by:

- `axiaoxin-com/investool`
- Repository: https://github.com/axiaoxin-com/investool
- License: Apache License 2.0

TradeHub's implementation is a separate Go service and adapts the functional
ideas into TradeHub's API and data model. If source snippets or substantially
similar logic are introduced later, preserve the Apache-2.0 license notice for
those files.

Additional engineering reference:

- `hzm0321/real-time-fund`
- Repository: https://github.com/hzm0321/real-time-fund
- License: GNU Affero General Public License v3.0

TradeHub only adopts high-level engineering ideas from this project, such as
fund-to-sector mapping, sector `secid` lookup, batched sector quote fetching,
tag recommendation, and sync-state separation. The AGPL upstream source code,
React components, Supabase schema implementation, and complete CSV datasets are
not vendored into this MIT-licensed repository.
