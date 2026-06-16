package ncbi

import (
	"strings"

	"github.com/KiriKirby/phytozome-go/internal/model"
)

const (
	searchTypeCandidatePrefix = "ncbi_searchtype:"

	ResultDomainSequenceRecord    = "sequence-record"
	ResultDomainGeneRecord        = "gene-record"
	ResultDomainGenomeResource    = "genome-resource"
	ResultDomainSampleProject     = "sample-project"
	ResultDomainVariantClinical   = "variant-clinical"
	ResultDomainLiterature        = "literature-reference"
	ResultDomainTaxonomyReference = "taxonomy-reference"
	ResultDomainChemicalBioassay  = "chemical-bioassay"
	ResultDomainCatalogReference  = "catalog-reference"
	ResultDomainAnnotationRecord  = "annotation-record"
)

type SearchType struct {
	ID              string
	EntrezDB        string
	Label           string
	Description     string
	ResultDomain    string
	RecordType      string
	ShowsSymbolName bool
	ShowsGeneLocus  bool
	SupportsWide    bool
	SearchEnabled   bool
}

var searchTypes = []SearchType{
	{ID: "protein", EntrezDB: "protein", Label: "Protein", Description: "protein sequence records through NCBI E-utilities", ResultDomain: ResultDomainSequenceRecord, RecordType: "protein", ShowsSymbolName: true, ShowsGeneLocus: true, SearchEnabled: true},
	{ID: "gene", EntrezDB: "gene", Label: "Gene", Description: "gene records through NCBI E-utilities", ResultDomain: ResultDomainGeneRecord, RecordType: "gene", ShowsSymbolName: true, ShowsGeneLocus: true, SearchEnabled: true},
	{ID: "nuccore", EntrezDB: "nuccore", Label: "Nucleotide (nuccore)", Description: "nucleotide sequence records through NCBI E-utilities", ResultDomain: ResultDomainSequenceRecord, RecordType: "nucleotide", SearchEnabled: true},
	{ID: "nucleotide", EntrezDB: "nucleotide", Label: "Nucleotide", Description: "legacy nucleotide Entrez view through NCBI E-utilities", ResultDomain: ResultDomainSequenceRecord, RecordType: "nucleotide"},
	{ID: "ipg", EntrezDB: "ipg", Label: "Identical Protein Groups", Description: "identical protein groups through NCBI E-utilities", ResultDomain: ResultDomainSequenceRecord, RecordType: "identical protein group"},
	{ID: "structure", EntrezDB: "structure", Label: "Structure", Description: "macromolecular structure records through NCBI E-utilities", ResultDomain: ResultDomainSequenceRecord, RecordType: "structure"},
	{ID: "genome", EntrezDB: "genome", Label: "Genome", Description: "genome resources through NCBI E-utilities", ResultDomain: ResultDomainGenomeResource, RecordType: "genome"},
	{ID: "annotinfo", EntrezDB: "annotinfo", Label: "AnnotInfo", Description: "annotation information records through NCBI E-utilities", ResultDomain: ResultDomainAnnotationRecord, RecordType: "annotation info"},
	{ID: "assembly", EntrezDB: "assembly", Label: "Assembly", Description: "assembly resources through NCBI E-utilities", ResultDomain: ResultDomainGenomeResource, RecordType: "assembly", SearchEnabled: true},
	{ID: "bioproject", EntrezDB: "bioproject", Label: "BioProject", Description: "BioProject records through NCBI E-utilities", ResultDomain: ResultDomainSampleProject, RecordType: "bioproject", SearchEnabled: true},
	{ID: "biosample", EntrezDB: "biosample", Label: "BioSample", Description: "BioSample records through NCBI E-utilities", ResultDomain: ResultDomainSampleProject, RecordType: "biosample", SearchEnabled: true},
	{ID: "blastdbinfo", EntrezDB: "blastdbinfo", Label: "BLAST DB Info", Description: "BLAST database metadata through NCBI E-utilities", ResultDomain: ResultDomainGenomeResource, RecordType: "blast database"},
	{ID: "books", EntrezDB: "books", Label: "Books", Description: "Bookshelf references through NCBI E-utilities", ResultDomain: ResultDomainLiterature, RecordType: "book chapter"},
	{ID: "cdd", EntrezDB: "cdd", Label: "Conserved Domains", Description: "conserved domain records through NCBI E-utilities", ResultDomain: ResultDomainSequenceRecord, RecordType: "conserved domain"},
	{ID: "clinvar", EntrezDB: "clinvar", Label: "ClinVar", Description: "clinical variant records through NCBI E-utilities", ResultDomain: ResultDomainVariantClinical, RecordType: "clinical variant", SearchEnabled: true},
	{ID: "gap", EntrezDB: "gap", Label: "dbGaP", Description: "dbGaP records through NCBI E-utilities", ResultDomain: ResultDomainVariantClinical, RecordType: "dbGaP study"},
	{ID: "gapplus", EntrezDB: "gapplus", Label: "dbGaP Plus", Description: "dbGaP plus records through NCBI E-utilities", ResultDomain: ResultDomainVariantClinical, RecordType: "dbGaP plus study"},
	{ID: "grasp", EntrezDB: "grasp", Label: "GRASP", Description: "GRASP phenotype/genotype records through NCBI E-utilities", ResultDomain: ResultDomainVariantClinical, RecordType: "GRASP record"},
	{ID: "dbvar", EntrezDB: "dbvar", Label: "dbVar", Description: "structural variation records through NCBI E-utilities", ResultDomain: ResultDomainVariantClinical, RecordType: "structural variant", SearchEnabled: true},
	{ID: "gds", EntrezDB: "gds", Label: "GEO DataSets", Description: "GEO DataSets through NCBI E-utilities", ResultDomain: ResultDomainVariantClinical, RecordType: "GEO dataset"},
	{ID: "geoprofiles", EntrezDB: "geoprofiles", Label: "GEO Profiles", Description: "GEO Profiles through NCBI E-utilities", ResultDomain: ResultDomainVariantClinical, RecordType: "GEO profile"},
	{ID: "medgen", EntrezDB: "medgen", Label: "MedGen", Description: "medical genetics concepts through NCBI E-utilities", ResultDomain: ResultDomainVariantClinical, RecordType: "MedGen concept", SearchEnabled: true},
	{ID: "mesh", EntrezDB: "mesh", Label: "MeSH", Description: "MeSH vocabulary records through NCBI E-utilities", ResultDomain: ResultDomainCatalogReference, RecordType: "MeSH term"},
	{ID: "nlmcatalog", EntrezDB: "nlmcatalog", Label: "NLM Catalog", Description: "NLM Catalog records through NCBI E-utilities", ResultDomain: ResultDomainCatalogReference, RecordType: "catalog record"},
	{ID: "omim", EntrezDB: "omim", Label: "OMIM", Description: "OMIM concept records through NCBI E-utilities", ResultDomain: ResultDomainVariantClinical, RecordType: "OMIM record", SearchEnabled: true},
	{ID: "orgtrack", EntrezDB: "orgtrack", Label: "OrgTrack", Description: "organism tracking records through NCBI E-utilities", ResultDomain: ResultDomainTaxonomyReference, RecordType: "organism tracking record"},
	{ID: "pmc", EntrezDB: "pmc", Label: "PMC", Description: "PubMed Central article records through NCBI E-utilities", ResultDomain: ResultDomainLiterature, RecordType: "PMC article"},
	{ID: "proteinclusters", EntrezDB: "proteinclusters", Label: "Protein Clusters", Description: "protein cluster records through NCBI E-utilities", ResultDomain: ResultDomainSequenceRecord, RecordType: "protein cluster"},
	{ID: "pcassay", EntrezDB: "pcassay", Label: "PubChem BioAssay", Description: "PubChem BioAssay records through NCBI E-utilities", ResultDomain: ResultDomainChemicalBioassay, RecordType: "bioassay"},
	{ID: "protfam", EntrezDB: "protfam", Label: "Protein Family Models", Description: "protein family model records through NCBI E-utilities", ResultDomain: ResultDomainSequenceRecord, RecordType: "protein family model"},
	{ID: "pccompound", EntrezDB: "pccompound", Label: "PubChem Compound", Description: "PubChem Compound records through NCBI E-utilities", ResultDomain: ResultDomainChemicalBioassay, RecordType: "compound"},
	{ID: "pcsubstance", EntrezDB: "pcsubstance", Label: "PubChem Substance", Description: "PubChem Substance records through NCBI E-utilities", ResultDomain: ResultDomainChemicalBioassay, RecordType: "substance"},
	{ID: "pubmed", EntrezDB: "pubmed", Label: "PubMed", Description: "PubMed references through NCBI E-utilities", ResultDomain: ResultDomainLiterature, RecordType: "PubMed article"},
	{ID: "seqannot", EntrezDB: "seqannot", Label: "SeqAnnot", Description: "sequence annotation records through NCBI E-utilities", ResultDomain: ResultDomainAnnotationRecord, RecordType: "sequence annotation"},
	{ID: "snp", EntrezDB: "snp", Label: "SNP", Description: "SNP records through NCBI E-utilities", ResultDomain: ResultDomainVariantClinical, RecordType: "SNP", SearchEnabled: true},
	{ID: "sra", EntrezDB: "sra", Label: "SRA", Description: "Sequence Read Archive records through NCBI E-utilities", ResultDomain: ResultDomainSampleProject, RecordType: "SRA run or study", SearchEnabled: true},
	{ID: "taxonomy", EntrezDB: "taxonomy", Label: "Taxonomy", Description: "taxonomy records through NCBI E-utilities", ResultDomain: ResultDomainTaxonomyReference, RecordType: "taxonomy record", SearchEnabled: true},
	{ID: "biocollections", EntrezDB: "biocollections", Label: "BioCollections", Description: "biological collection records through NCBI E-utilities", ResultDomain: ResultDomainSampleProject, RecordType: "biocollection"},
	{ID: "gtr", EntrezDB: "gtr", Label: "GTR", Description: "Genetic Testing Registry records through NCBI E-utilities", ResultDomain: ResultDomainVariantClinical, RecordType: "genetic test", SearchEnabled: true},
}

var searchTypeByID = func() map[string]SearchType {
	out := make(map[string]SearchType, len(searchTypes))
	for _, spec := range searchTypes {
		out[strings.ToLower(strings.TrimSpace(spec.ID))] = spec
	}
	return out
}()

func SearchTypes() []SearchType {
	return append([]SearchType(nil), searchTypes...)
}

func SearchableSearchTypes() []SearchType {
	out := make([]SearchType, 0, len(searchTypes))
	for _, spec := range searchTypes {
		if spec.SearchEnabled {
			out = append(out, spec)
		}
	}
	return out
}

func SearchTypeByID(id string) SearchType {
	if spec, ok := searchTypeByID[strings.ToLower(strings.TrimSpace(id))]; ok {
		return spec
	}
	return searchTypeByID["protein"]
}

func SearchTypeFromSpeciesCandidate(candidate model.SpeciesCandidate) SearchType {
	id := strings.TrimSpace(strings.TrimPrefix(candidate.JBrowseName, searchTypeCandidatePrefix))
	if id == "" {
		id = strings.TrimSpace(candidate.SearchAlias)
	}
	return SearchTypeByID(id)
}

func SearchTypeIDFromSpeciesCandidate(candidate model.SpeciesCandidate) string {
	return SearchTypeFromSpeciesCandidate(candidate).ID
}

func SyntheticSpeciesCandidate(id string) model.SpeciesCandidate {
	spec := SearchTypeByID(id)
	return model.SpeciesCandidate{
		ProteomeID:  0,
		JBrowseName: searchTypeCandidatePrefix + spec.ID,
		GenomeLabel: "NCBI " + spec.Label,
		CommonName:  spec.ResultDomain,
		SearchAlias: spec.ID,
	}
}

func ResultDomainFromKeywordRows(rows []model.KeywordResultRow) string {
	for _, row := range rows {
		if row.ExtraColumns == nil {
			continue
		}
		if value := strings.TrimSpace(row.ExtraColumns["ncbi_result_domain"]); value != "" {
			return value
		}
	}
	return ""
}
