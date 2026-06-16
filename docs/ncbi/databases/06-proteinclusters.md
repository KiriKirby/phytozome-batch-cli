# `proteinclusters`

## Role

- official menu name: `Protein Clusters`
- primary result domain: `sequence-group`
- current fit: good for cluster-centric metadata review

## Why users choose it

- inspect curated or grouped protein families
- move from one hit to a broader cluster context

## Retrieval plan

- `ESearch(db=proteinclusters)`
- `ESummary` for main rows
- later specialized linked exploration to protein/gene/genome/publication

## Important extractions

- cluster accession or ID
- title/description
- representative/member summary
- organism or taxonomic span
- publication support when available

## Important jumps

- `proteinclusters -> gene`
- `proteinclusters -> genome`
- `proteinclusters -> nuccore`
- `proteinclusters -> protein`
- `proteinclusters -> pubmed`

## Table plan

Display:

- `search_term`, `search_type`, `label_name`, `cluster_id`, `description`, `genome`, `member_count`

Detail/export emphasis:

- representative member
- member count
- cluster notes
- linked gene/protein references

## Special prompts

- cluster rows should offer “jump to member proteins” rather than pretending to be sequence-export rows by default
