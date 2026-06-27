# `grasp`

## Role

- official menu name: `grasp`
- primary result domain: `variant-clinical`
- current fit: specialist support mode

## Why users choose it

- phenotype/genotype association reference searching

## Retrieval plan

- `ESearch(db=grasp)`
- `ESummary` only
- current live `EInfo` exposes no link graph

## Important extractions

- record ID
- title
- trait summary
- source/date context

## Table plan

Display:

- `search_term`, `search_type`, `grasp_id`, `title`, `trait_summary`

Detail/export emphasis:

- source/date
- any phenotype context surfaced in summary

## Special prompts

- label clearly as specialist association metadata
