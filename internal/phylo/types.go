package phylo

import "time"

const (
	PHgoDisplayNameSource    = "phgo_label_format"
	YTDisplayNameSource      = "yt_label_format"
	YTV2DisplayNameSource    = "yt_v2_label_format"
	DefaultDisplayNameSource = PHgoDisplayNameSource
	DefaultAlignmentMethod   = AlignmentClustalW
	DefaultTreeMethod        = TreeNeighborJoining
	DefaultConversionTarget  = ConversionTargetProtein
)

type AlignmentMethod string

const (
	AlignmentClustalW       AlignmentMethod = "clustalw"
	AlignmentClustalWCodons AlignmentMethod = "clustalw_codons"
	AlignmentMUSCLE         AlignmentMethod = "muscle"
	AlignmentMUSCLECodons   AlignmentMethod = "muscle_codons"
)

type TreeMethod string

const (
	TreeNeighborJoining   TreeMethod = "neighbor_joining"
	TreeMinimumEvolution  TreeMethod = "minimum_evolution"
	TreeUPGMA             TreeMethod = "upgma"
	TreeMaximumLikelihood TreeMethod = "maximum_likelihood"
	TreeMaximumParsimony  TreeMethod = "maximum_parsimony"
)

type SequenceKind string

const (
	SequenceProtein    SequenceKind = "protein"
	SequenceNucleotide SequenceKind = "nucleotide"
	SequenceUnknown    SequenceKind = "unknown"
)

type ConversionTarget string

const (
	ConversionTargetDNA     ConversionTarget = "dna"
	ConversionTargetProtein ConversionTarget = "protein"
)

type TreeSettings struct {
	DisplayNameSource        string            `json:"display_name_source"`
	ShowCanvasCoordinates    bool              `json:"show_canvas_coordinates"`
	ShowCanvasCoordinatesSet bool              `json:"show_canvas_coordinates_set,omitempty"`
	ConversionTarget         ConversionTarget  `json:"conversion_target"`
	ConversionSkipUnselect   bool              `json:"conversion_skip_unselect,omitempty"`
	AlignmentMethod          AlignmentMethod   `json:"alignment_method"`
	AlignmentParams          map[string]string `json:"alignment_params,omitempty"`
	TreeMethod               TreeMethod        `json:"tree_method"`
	TreeParams               map[string]string `json:"tree_params,omitempty"`
}

type ParameterKind string

const (
	ParameterSection  ParameterKind = "section"
	ParameterString   ParameterKind = "string"
	ParameterInteger  ParameterKind = "integer"
	ParameterFloat    ParameterKind = "float"
	ParameterPicklist ParameterKind = "picklist"
)

type ParameterDefinition struct {
	ID          string        `json:"id"`
	Label       string        `json:"label"`
	Kind        ParameterKind `json:"kind"`
	Default     string        `json:"default,omitempty"`
	Options     []string      `json:"options,omitempty"`
	ReadOnly    bool          `json:"read_only,omitempty"`
	Applicable  bool          `json:"applicable"`
	IndentLevel int           `json:"indent_level,omitempty"`
	Min         string        `json:"min,omitempty"`
	Max         string        `json:"max,omitempty"`
	Increment   string        `json:"increment,omitempty"`
	Precision   int           `json:"precision,omitempty"`
}

type MethodDefinition struct {
	ID             string                `json:"id"`
	Label          string                `json:"label"`
	RuntimeMethod  string                `json:"runtime_method,omitempty"`
	SequenceKinds  []SequenceKind        `json:"sequence_kinds"`
	CodingRequired bool                  `json:"coding_required,omitempty"`
	Parameters     []ParameterDefinition `json:"parameters"`
}

func DefaultTreeSettings() TreeSettings {
	return TreeSettings{
		DisplayNameSource:        DefaultDisplayNameSource,
		ShowCanvasCoordinates:    true,
		ShowCanvasCoordinatesSet: true,
		ConversionTarget:         DefaultConversionTarget,
		ConversionSkipUnselect:   true,
		AlignmentMethod:          DefaultAlignmentMethod,
		AlignmentParams:          map[string]string{},
		TreeMethod:               DefaultTreeMethod,
		TreeParams:               map[string]string{},
	}
}

type InputRecord struct {
	TaxonID         string            `json:"taxon_id"`
	DisplayName     string            `json:"display_name"`
	BaseDisplayName string            `json:"base_display_name,omitempty"`
	DisplayPrefix   string            `json:"display_prefix,omitempty"`
	SourceType      string            `json:"source_type"`
	OriginalHead    string            `json:"original_head"`
	Sequence        string            `json:"-"`
	SequenceKind    SequenceKind      `json:"sequence_kind"`
	CanvasItem      string            `json:"canvas_item"`
	CanvasItemIndex int               `json:"canvas_item_index"`
	CanvasRow       int               `json:"canvas_row"`
	TableValues     map[string]string `json:"table_values,omitempty"`
	RowFingerprint  string            `json:"row_fingerprint"`
}

type Metadata struct {
	SchemaVersion         int               `json:"schema_version"`
	GeneratedAt           time.Time         `json:"generated_at"`
	DisplayNameSource     string            `json:"display_name_source"`
	TreeComputationSource string            `json:"tree_computation_source,omitempty"`
	SequenceKind          SequenceKind      `json:"sequence_kind,omitempty"`
	ConversionTarget      ConversionTarget  `json:"conversion_target,omitempty"`
	AlignmentMethod       AlignmentMethod   `json:"alignment_method,omitempty"`
	TreeMethod            TreeMethod        `json:"tree_method,omitempty"`
	AlignmentParams       map[string]string `json:"alignment_params,omitempty"`
	TreeParams            map[string]string `json:"tree_params,omitempty"`
	TreeCount             int               `json:"tree_count,omitempty"`
	Records               []InputRecord     `json:"records"`
}

type MSASelectionRow struct {
	TaxonID         string `json:"taxon_id"`
	DisplayName     string `json:"display_name,omitempty"`
	DisplayPrefix   string `json:"display_prefix,omitempty"`
	DisplayLabel    string `json:"display_label,omitempty"`
	CanvasItemIndex int    `json:"canvas_item_index,omitempty"`
	CanvasRow       int    `json:"canvas_row,omitempty"`
	Index           int    `json:"index"`
	State           string `json:"state"`
}

type MSASequenceState struct {
	TaxonID     string `json:"taxon_id"`
	DisplayName string `json:"display_name,omitempty"`
	Description string `json:"description,omitempty"`
	Sequence    string `json:"sequence,omitempty"`
	Index       int    `json:"index,omitempty"`
}

type MSAState struct {
	SchemaVersion int                `json:"schema_version"`
	UpdatedAt     time.Time          `json:"updated_at,omitempty"`
	Rows          []MSASelectionRow  `json:"rows,omitempty"`
	Sequences     []MSASequenceState `json:"sequences,omitempty"`
	ViewerState   map[string]any     `json:"viewer_state,omitempty"`
	Settings      map[string]any     `json:"settings,omitempty"`
	Annotations   []map[string]any   `json:"annotations,omitempty"`
	Features      []map[string]any   `json:"features,omitempty"`
	Groups        []map[string]any   `json:"groups,omitempty"`
	Markers       []map[string]any   `json:"markers,omitempty"`
}

type ViewerPayload struct {
	SchemaVersion int       `json:"schema_version"`
	SessionID     string    `json:"session_id"`
	Title         string    `json:"title,omitempty"`
	UpdatedAt     time.Time `json:"updated_at"`
	Newick        string    `json:"newick"`
	AlignedFASTA  string    `json:"aligned_fasta,omitempty"`
	Metadata      Metadata  `json:"metadata"`
}

type Fingerprints struct {
	Input     string `json:"input"`
	Alignment string `json:"alignment"`
	Tree      string `json:"tree"`
	Preview   string `json:"preview"`
}

type RuntimeSkippedRecord struct {
	TaxonID   string `json:"taxon_id"`
	ItemTitle string `json:"item_title"`
	RowIndex  int    `json:"row_index"`
	Reason    string `json:"reason"`
}

type RunManifest struct {
	SchemaVersion   int          `json:"schema_version"`
	CreatedAt       time.Time    `json:"created_at"`
	Settings        TreeSettings `json:"settings"`
	Fingerprints    Fingerprints `json:"fingerprints"`
	InputFASTA      string       `json:"input_fasta"`
	MetadataJSON    string       `json:"metadata_json"`
	RuntimeRequest  string       `json:"runtime_request,omitempty"`
	RuntimeResponse string       `json:"runtime_response,omitempty"`
	AlignedFASTA    string       `json:"aligned_fasta,omitempty"`
	NewickPath      string       `json:"newick_path,omitempty"`
	LogPath         string       `json:"log_path,omitempty"`
}

type RunResult struct {
	Plan           RunPlan
	ArtifactDir    string
	StdoutPath     string
	StderrPath     string
	SummaryPath    string
	SelectedNewick string
	ErrorText      string
	SkippedRecords []RuntimeSkippedRecord
	Reused         bool
}
