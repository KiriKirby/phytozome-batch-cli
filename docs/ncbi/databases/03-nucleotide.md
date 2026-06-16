# `nucleotide`

## Role

- official menu name: `Nucleotide`
- primary result domain: `sequence-record`
- current fit: compatibility/specialist mode, not the preferred primary nucleotide entry

## Why users choose it

- legacy Entrez behavior compatibility
- explicit user expectation that differs from `nuccore`

## Design stance

- `nuccore` should remain the main nucleotide-facing user option
- expose `nucleotide` only if the UI can explain why it exists separately
- if exposed, label it clearly as a legacy/compatibility NCBI view

## Retrieval plan

- same baseline as `nuccore` for `ESearch` and `ESummary`
- no assumption that it supports the same live link graph as `nuccore`
- avoid building major features on this token before proving real user value

## Table plan

Display:

- same baseline as `nuccore`

Detail/export emphasis:

- keep the exact Entrez database token visible in metadata so users can tell they did not search `nuccore`

## Special prompts

- none beyond the UI explanation that `nuccore` is usually the better general-purpose choice
