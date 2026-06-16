# Per-Database NCBI Specialization

This directory contains one design document for every currently indexed Entrez database token returned by live `EInfo`.

Each file is meant to answer the same implementation questions:

- what this database represents in NCBI terms
- how well it fits the current `phytozome GO` workflow
- which `ESearch`, `ESummary`, `EFetch`, and `ELink` patterns matter
- which table columns belong in display/detail/export
- which update, redirect, or jump prompts are specific to this database

Read together with:

- [../09-live-einfo-2026-06-13-summary.md](../09-live-einfo-2026-06-13-summary.md)
- [../10-database-family-deep-specialization.md](../10-database-family-deep-specialization.md)
- [../11-elink-jump-patterns.md](../11-elink-jump-patterns.md)
- [../12-efetch-extraction-plans.md](../12-efetch-extraction-plans.md)
- [../13-update-replacement-and-redirect-strategy.md](../13-update-replacement-and-redirect-strategy.md)
