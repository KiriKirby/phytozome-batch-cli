# `gap`

## Role

- official menu name: `dbGaP`
- primary result domain: `variant-clinical`
- current fit: specialist but important provenance/clinical context

## Why users choose it

- study/accession search in dbGaP
- project-level variant/phenotype context

## Retrieval plan

- `ESearch(db=gap)`
- `ESummary` for rows
- no assumption of user-open export parity because access policy matters

## Important extractions

- dbGaP accession
- title
- study type
- ancestry/population context when present
- access status

## Important jumps

- `gap -> bioproject`
- `gap -> biosample`
- `gap -> dbvar`
- `gap -> gds`
- `gap -> gene`
- `gap -> medgen`
- `gap -> snp`

## Table plan

Display:

- `search_term`, `search_type`, `gap_accession`, `title`, `study_type`, `access_status`

Detail/export emphasis:

- ancestry / disease / study design context
- linked gene/variant/project context

## Special prompts

- show access-policy and restricted-use caveats clearly
