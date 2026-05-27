// The contents of this file are subject to the Common Public Attribution License Version 1.0 (CPAL-1.0);
// you may not use this file except in compliance with the License. You may obtain a copy of the License at
// https://opensource.org/license/CPAL-1.0. Software distributed under the License is distributed on an "AS IS"
// basis, WITHOUT WARRANTY OF ANY KIND, either express or implied. The Original Code is phytozome GO. The
// Initial Developer is wangsychn. All portions of the code written by wangsychn are Copyright (c) 2026
// wangsychn. All Rights Reserved. Contributor(s): .

package workflow

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/KiriKirby/phytozome-go/internal/appfs"
	"github.com/KiriKirby/phytozome-go/internal/model"
	"github.com/KiriKirby/phytozome-go/internal/prompt"
	"github.com/KiriKirby/phytozome-go/internal/sessionsnapshot"
	"github.com/KiriKirby/phytozome-go/internal/tui"
)

type canvasLaunchState struct {
	Items         []model.CanvasItem
	CurrentItem   int
	NextNumericID int
	ImportedFrom  string
	SaveBaseName  string
}

func (w *BlastWizard) runCanvasMode(ctx context.Context, state canvasLaunchState) error {
	if state.NextNumericID <= 0 {
		state.NextNumericID = nextCanvasNumericID(state.Items)
	}
	for {
		selection, err := w.prompt.SelectCanvas(state.Items, state.CurrentItem, state.NextNumericID, "canvas", prompt.ErrBackToDatabaseSelection)
		if err != nil {
			return err
		}
		state.Items = cloneCanvasItems(selection.Items)
		state.CurrentItem = selection.CurrentItem
		state.NextNumericID = selection.NextNumericID
		switch strings.TrimSpace(selection.Action) {
		case "", "view":
			if !selection.GenerateFile {
				continue
			}
			if len(state.Items) == 0 {
				if infoErr := w.showInfo("Canvas", "Canvas is empty. Add items before saving a snapshot.", prompt.ErrBackToDatabaseSelection); infoErr != nil {
					return infoErr
				}
				continue
			}
			saveSettings, saveErr := w.prompt.CanvasSaveSettings(firstNonEmpty(state.SaveBaseName, canvasDefaultSaveName(state.Items)), prompt.ErrBackToDatabaseSelection)
			if saveErr != nil {
				if errors.Is(saveErr, prompt.ErrBackToDatabaseSelection) {
					continue
				}
				return saveErr
			}
			outputDir, outErr := appfs.OutputDir()
			if outErr != nil {
				return outErr
			}
			if err := w.writeCanvasSessionSnapshot(state, exportSettings{BaseName: saveSettings.BaseName, OutputDir: outputDir, WriteSession: saveSettings.WriteSession}); err != nil {
				return err
			}
			if infoErr := w.showInfo("Canvas", "Canvas snapshot saved to output.", prompt.ErrBackToDatabaseSelection); infoErr != nil {
				return infoErr
			}
		case "delete_item":
			if state.CurrentItem >= 0 && state.CurrentItem < len(state.Items) {
				state.Items = append(state.Items[:state.CurrentItem], state.Items[state.CurrentItem+1:]...)
				if state.CurrentItem >= len(state.Items) {
					state.CurrentItem = len(state.Items) - 1
				}
				if state.CurrentItem < 0 {
					state.CurrentItem = 0
				}
			}
		case "rename_item":
			if len(state.Items) == 0 || state.CurrentItem < 0 || state.CurrentItem >= len(state.Items) {
				continue
			}
			name, nameErr := w.prompt.CanvasRenameInput(state.Items[state.CurrentItem].Title)
			if nameErr != nil {
				if nameErr == prompt.ErrBackToRowSelection {
					continue
				}
				return nameErr
			}
			state.Items[state.CurrentItem].Title = strings.TrimSpace(name)
			state.SaveBaseName = canvasDefaultSaveName(state.Items)
		case "add_item":
			input, inputErr := w.prompt.CanvasAddItemInput()
			if inputErr != nil {
				if inputErr == prompt.ErrBackToRowSelection {
					continue
				}
				return inputErr
			}
			items, itemErr := w.canvasItemsFromInput(ctx, strings.TrimSpace(input), state.NextNumericID)
			if itemErr != nil {
				if infoErr := w.showInfo("Canvas", itemErr.Error(), prompt.ErrBackToDatabaseSelection); infoErr != nil {
					return infoErr
				}
				continue
			}
			state.Items = append(state.Items, items...)
			state.CurrentItem = len(state.Items) - 1
			state.NextNumericID = nextCanvasNumericID(state.Items)
			state.SaveBaseName = canvasDefaultSaveName(state.Items)
		case "add_rows":
			if len(state.Items) == 0 || state.CurrentItem < 0 || state.CurrentItem >= len(state.Items) {
				continue
			}
			input, inputErr := w.prompt.CanvasAddRowsInput()
			if inputErr != nil {
				if inputErr == prompt.ErrBackToRowSelection {
					continue
				}
				return inputErr
			}
			rows, rowErr := w.canvasRowsFromFastaInput(strings.TrimSpace(input))
			if rowErr != nil {
				if infoErr := w.showInfo("Canvas", rowErr.Error(), prompt.ErrBackToDatabaseSelection); infoErr != nil {
					return infoErr
				}
				continue
			}
			item := &state.Items[state.CurrentItem]
			rows = assignCanvasRowNumbers(rows, nextCanvasRowNumber(item.Rows))
			item.Rows = append(item.Rows, rows...)
			item.Selected = normalizeCanvasSelection(item.Selected, len(item.Rows))
			for i := len(item.Rows) - len(rows); i < len(item.Rows); i++ {
				if i >= 0 && i < len(item.Selected) {
					item.Selected[i] = true
				}
			}
			updateCanvasItemSubtitle(item)
		case "delete_rows":
			if len(state.Items) == 0 || state.CurrentItem < 0 || state.CurrentItem >= len(state.Items) {
				continue
			}
			item := &state.Items[state.CurrentItem]
			selected := normalizeCanvasSelection(item.Selected, len(item.Rows))
			if countTrueBools(selected) == 0 {
				continue
			}
			nextRows := make([]model.CanvasRow, 0, len(item.Rows))
			nextSelected := make([]bool, 0, len(item.Rows))
			for i, row := range item.Rows {
				if i < len(selected) && selected[i] {
					continue
				}
				nextRows = append(nextRows, row)
				nextSelected = append(nextSelected, false)
			}
			item.Rows = nextRows
			item.Selected = nextSelected
			updateCanvasItemSubtitle(item)
		default:
			continue
		}
	}
}

func canvasDefaultSaveName(items []model.CanvasItem) string {
	if len(items) == 0 {
		return "canvas"
	}
	for _, item := range items {
		if value := strings.TrimSpace(item.Title); value != "" {
			return value
		}
	}
	return "canvas"
}

func nextCanvasNumericID(items []model.CanvasItem) int {
	maxID := 0
	for _, item := range items {
		name := strings.TrimSpace(item.Title)
		if name == "" {
			continue
		}
		value := 0
		for _, ch := range name {
			if ch < '0' || ch > '9' {
				value = 0
				break
			}
			value = value*10 + int(ch-'0')
		}
		if value > maxID {
			maxID = value
		}
	}
	if maxID <= 0 {
		return 1
	}
	return maxID + 1
}

func (w *BlastWizard) canvasItemsFromInput(ctx context.Context, input string, index int) ([]model.CanvasItem, error) {
	if strings.TrimSpace(input) == "" {
		return nil, fmt.Errorf("canvas input cannot be empty")
	}
	if snapshot, snapshotName, ok, err := w.tryLoadCanvasSnapshotInput(ctx, input); err != nil {
		return nil, err
	} else if ok {
		items := cloneCanvasItems(snapshotItemsFromSnapshot(snapshot))
		if len(items) == 0 {
			return nil, fmt.Errorf("snapshot has no canvas tables to import")
		}
		if len(items) == 1 && strings.TrimSpace(snapshotName) != "" {
			items[0].Title = strings.TrimSpace(snapshotName)
		}
		if len(items) > 1 {
			indices, err := w.prompt.SelectCanvasSnapshotItems(items, filepath.Base(strings.TrimSpace(input)))
			if err != nil {
				return nil, err
			}
			if len(indices) == 0 {
				return nil, fmt.Errorf("no canvas tables were selected")
			}
			filtered := make([]model.CanvasItem, 0, len(indices))
			for _, idx := range indices {
				if idx >= 0 && idx < len(items) {
					filtered = append(filtered, items[idx])
				}
			}
			items = filtered
		}
		for i := range items {
			items[i].Selected = normalizeCanvasSelection(items[i].Selected, len(items[i].Rows))
			updateCanvasItemSubtitle(&items[i])
		}
		return items, nil
	}
	rows, err := w.canvasRowsFromFastaInput(input)
	if err != nil {
		return nil, err
	}
	title := fmt.Sprintf("%d", index)
	item := model.CanvasItem{
		Title:        title,
		Kind:         model.CanvasKindFasta,
		Rows:         assignCanvasRowNumbers(rows, 1),
		Selected:     make([]bool, len(rows)),
		ImportedFrom: "fasta",
	}
	for i := range item.Selected {
		item.Selected[i] = true
	}
	updateCanvasItemSubtitle(&item)
	return []model.CanvasItem{item}, nil
}

func (w *BlastWizard) tryLoadCanvasSnapshotInput(ctx context.Context, input string) (sessionsnapshot.Snapshot, string, bool, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" || strings.Contains(trimmed, "\n") || strings.HasPrefix(trimmed, ">") {
		return sessionsnapshot.Snapshot{}, "", false, nil
	}
	path, err := sessionsnapshot.ResolveOpenPath(trimmed, mustOutputDir())
	if err != nil {
		return sessionsnapshot.Snapshot{}, "", false, nil
	}
	snapshot, err := w.loadCanvasImportSnapshot(ctx, path)
	if err != nil {
		return sessionsnapshot.Snapshot{}, "", true, err
	}
	return snapshot, strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)), true, nil
}

func (w *BlastWizard) canvasRowsFromFastaInput(input string) ([]model.CanvasRow, error) {
	records := splitBlastInputRecords(input)
	if len(records) == 0 {
		return nil, fmt.Errorf("no FASTA records were found")
	}
	rows := make([]model.CanvasRow, 0, len(records))
	for _, record := range records {
		record = strings.TrimSpace(record)
		if record == "" {
			continue
		}
		if !strings.HasPrefix(record, ">") {
			return nil, fmt.Errorf("adding rows only supports FASTA input")
		}
		source, ok := parseFastaQuerySequenceInput(record)
		if !ok || source == nil {
			header, _ := splitFastaHeaderAndSequence(record)
			if header == "" {
				return nil, fmt.Errorf("invalid FASTA input near %q", oneLinePreview(record))
			}
			source = &model.QuerySequenceSource{
				Annotation:     strings.TrimSpace(header),
				SourceDatabase: "canvas",
			}
		}
		if strings.TrimSpace(source.SourceDatabase) == "" {
			source.SourceDatabase = "canvas"
		}
		rows = append(rows, model.CanvasRow{
			Kind:  model.CanvasKindFasta,
			FASTA: source,
		})
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("no FASTA rows could be created")
	}
	return rows, nil
}

func normalizeCanvasSelection(selected []bool, size int) []bool {
	if size <= 0 {
		return nil
	}
	if len(selected) == size {
		return append([]bool(nil), selected...)
	}
	out := make([]bool, size)
	copy(out, selected)
	return out
}

func updateCanvasItemSubtitle(item *model.CanvasItem) {
	if item == nil {
		return
	}
	item.Subtitle = fmt.Sprintf("%d/%d lines", len(item.Rows), countTrueBools(normalizeCanvasSelection(item.Selected, len(item.Rows))))
}

func nextCanvasRowNumber(rows []model.CanvasRow) int {
	maxNumber := 0
	for i, row := range rows {
		value := row.RowNumber
		if value <= 0 {
			value = i + 1
		}
		if value > maxNumber {
			maxNumber = value
		}
	}
	return maxNumber + 1
}

func assignCanvasRowNumbers(rows []model.CanvasRow, start int) []model.CanvasRow {
	if start <= 0 {
		start = 1
	}
	out := make([]model.CanvasRow, len(rows))
	copy(out, rows)
	next := start
	for i := range out {
		if out[i].RowNumber > 0 {
			if out[i].RowNumber >= next {
				next = out[i].RowNumber + 1
			}
			continue
		}
		out[i].RowNumber = next
		next++
	}
	return out
}

type canvasRowView struct {
	Number     int
	SourceType string
	Head       string
	Species    string
	LabelName  string
	GeneID     string
	SourceGene string
}

func canvasHeadFromSource(source model.QuerySequenceSource, fallback string) string {
	if label := strings.TrimSpace(source.LabelName); label != "" {
		return label
	}
	if label := strings.TrimSpace(source.PreferredSequenceID); label != "" {
		return label
	}
	if label := strings.TrimSpace(source.ProteinID); label != "" {
		return label
	}
	if label := strings.TrimSpace(source.TranscriptID); label != "" {
		return label
	}
	if label := strings.TrimSpace(source.GeneID); label != "" {
		return label
	}
	if label := strings.TrimSpace(source.Annotation); label != "" {
		fields := strings.Fields(label)
		if len(fields) > 0 {
			return fields[0]
		}
	}
	return strings.TrimSpace(fallback)
}

func canvasRowViewFromRow(row model.CanvasRow) canvasRowView {
	switch row.Kind {
	case model.CanvasKindFasta:
		if row.FASTA != nil {
			return canvasRowView{
				SourceType: strings.TrimSpace(firstNonEmpty(row.FASTA.SourceDatabase, string(model.CanvasKindFasta))),
				Head:       canvasHeadFromSource(*row.FASTA, ""),
				Species:    strings.TrimSpace(row.FASTA.OrganismShort),
				LabelName:  strings.TrimSpace(row.FASTA.LabelName),
				GeneID:     strings.TrimSpace(row.FASTA.GeneID),
				SourceGene: strings.TrimSpace(row.FASTA.BlastSourceGeneID),
			}
		}
	case model.CanvasKindKeyword:
		if row.KeywordRow != nil {
			return canvasRowView{
				SourceType: strings.TrimSpace(firstNonEmpty(row.KeywordRow.SourceDatabase, string(model.CanvasKindKeyword))),
				Head:       strings.TrimSpace(firstNonEmpty(row.KeywordRow.SequenceHeaderLabel, row.KeywordRow.ProteinID, row.KeywordRow.TranscriptID, row.KeywordRow.GeneIdentifier)),
				Species:    strings.TrimSpace(row.KeywordRow.Genome),
				LabelName:  strings.TrimSpace(row.KeywordRow.LabelName),
				GeneID:     strings.TrimSpace(row.KeywordRow.GeneIdentifier),
			}
		}
	case model.CanvasKindBlast:
		if row.BlastRow != nil {
			return canvasRowView{
				SourceType: strings.TrimSpace(firstNonEmpty(row.BlastRow.SourceDatabase, string(model.CanvasKindBlast))),
				Head:       strings.TrimSpace(firstNonEmpty(row.BlastRow.SubjectID, row.BlastRow.Protein, row.BlastRow.SequenceID)),
				Species:    strings.TrimSpace(row.BlastRow.Species),
				LabelName:  strings.TrimSpace(row.BlastRow.LabelName),
				GeneID:     strings.TrimSpace(firstNonEmpty(row.BlastRow.SequenceID, row.BlastRow.TranscriptID, row.BlastRow.Protein)),
				SourceGene: strings.TrimSpace(row.BlastRow.BlastGeneID),
			}
		}
	}
	return canvasRowView{SourceType: string(row.Kind)}
}

func canvasRowsSorted(rows []model.CanvasRow) []model.CanvasRow {
	type indexed struct {
		row  model.CanvasRow
		view canvasRowView
		idx  int
	}
	indexedRows := make([]indexed, 0, len(rows))
	for i, row := range rows {
		indexedRows = append(indexedRows, indexed{row: row, view: canvasRowViewFromRow(row), idx: i})
	}
	sort.SliceStable(indexedRows, func(i, j int) bool {
		left, right := indexedRows[i].view, indexedRows[j].view
		if cmp := strings.Compare(strings.ToLower(left.Head), strings.ToLower(right.Head)); cmp != 0 {
			return cmp < 0
		}
		if cmp := strings.Compare(strings.ToLower(left.GeneID), strings.ToLower(right.GeneID)); cmp != 0 {
			return cmp < 0
		}
		return indexedRows[i].idx < indexedRows[j].idx
	})
	out := make([]model.CanvasRow, 0, len(rows))
	for _, item := range indexedRows {
		out = append(out, item.row)
	}
	return out
}

func (w *BlastWizard) writeCanvasSessionSnapshot(state canvasLaunchState, settings exportSettings) error {
	path := sessionsnapshot.DefaultFilePath(mustOutputDir(), firstNonEmpty(settings.BaseName, "canvas"))
	return w.writeSessionSnapshotWithProgress(path, "Writing canvas session snapshot...", func(ctx context.Context, update func(int, string)) (sessionsnapshot.Snapshot, error) {
		progress := safeProgress(update)
		progress(0, "Preparing canvas snapshot data...")
		progress(1, "Recording canvas review state...")
		progress(2, "Canvas snapshot data prepared.")
		return sessionsnapshot.Snapshot{
			Context: w.snapshotContext(string(ModeCanvas), "canvas-result", "Canvas"),
			Canvas: &sessionsnapshot.CanvasResultV1{
				Items:         snapshotCanvasItems(state.Items),
				CurrentItem:   state.CurrentItem,
				NextNumericID: state.NextNumericID,
				ImportedFrom:  state.ImportedFrom,
			},
			CanvasReview: &sessionsnapshot.CanvasReviewStateV1{
				SelectionState: w.prompt.SnapshotCanvasReviewState(canvasStateKey("canvas")),
			},
			ExportSettings: snapshotExportSettings(w.prompt.SnapshotExportSettings(), settings),
			Handoff:        w.snapshotHandoffState(),
			RuntimeCache:   w.snapshotRuntimeCache(),
		}, nil
	})
}

func canvasStateKey(title string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		title = "canvas"
	}
	return "canvas:" + title
}

func snapshotCanvasItems(items []model.CanvasItem) []sessionsnapshot.CanvasItemV1 {
	out := make([]sessionsnapshot.CanvasItemV1, len(items))
	for i := range items {
		out[i].Title = items[i].Title
		out[i].Subtitle = items[i].Subtitle
		out[i].Kind = items[i].Kind
		out[i].Selected = append([]bool(nil), items[i].Selected...)
		out[i].SourceLabel = items[i].SourceLabel
		out[i].ImportedFrom = items[i].ImportedFrom
		out[i].Rows = make([]sessionsnapshot.CanvasRowV1, len(items[i].Rows))
		for j := range items[i].Rows {
			out[i].Rows[j].RowNumber = items[i].Rows[j].RowNumber
			out[i].Rows[j].Kind = items[i].Rows[j].Kind
			if items[i].Rows[j].KeywordRow != nil {
				copyRow := *items[i].Rows[j].KeywordRow
				out[i].Rows[j].KeywordRow = &copyRow
			}
			if items[i].Rows[j].BlastRow != nil {
				copyRow := *items[i].Rows[j].BlastRow
				out[i].Rows[j].BlastRow = &copyRow
			}
			if items[i].Rows[j].FASTA != nil {
				copySource := *items[i].Rows[j].FASTA
				out[i].Rows[j].FASTA = &copySource
			}
		}
	}
	return out
}

func mustOutputDir() string {
	dir, err := appfs.OutputDir()
	if err != nil {
		return "."
	}
	return dir
}

func (w *BlastWizard) loadCanvasImportSnapshot(ctx context.Context, input string) (sessionsnapshot.Snapshot, error) {
	path, err := sessionsnapshot.ResolveOpenPath(input, mustOutputDir())
	if err != nil {
		return sessionsnapshot.Snapshot{}, err
	}
	if w.suppressTaskModals {
		return sessionsnapshot.ReadFile(path)
	}
	return tui.RunProgressTaskValueContext(tui.TaskPage{
		Path:        w.tuiPath("Startup", "Explore", "Canvas", "Import snapshot"),
		Title:       "Opening snapshot",
		Description: "Reading the frozen snapshot before importing its tables into the current canvas.",
		Initial:     "Opening snapshot...",
		Total:       2,
		CancelError: prompt.ErrBackToRowSelection,
	}, func(taskCtx context.Context, update func(int, string)) (sessionsnapshot.Snapshot, error) {
		progress := safeProgress(update)
		progress(0, "Reading snapshot...")
		snapshot, err := sessionsnapshot.ReadFile(path)
		if err != nil {
			return sessionsnapshot.Snapshot{}, err
		}
		progress(2, "Snapshot opened.")
		return snapshot, nil
	})
}

func snapshotItemsFromSnapshot(snapshot sessionsnapshot.Snapshot) []model.CanvasItem {
	if snapshot.Canvas != nil && len(snapshot.Canvas.Items) > 0 {
		return canvasItemsFromSnapshot(snapshot.Canvas.Items)
	}
	if snapshot.Keyword != nil {
		return []model.CanvasItem{canvasItemFromKeywordSnapshot(snapshot.Keyword)}
	}
	if snapshot.Blast != nil {
		return canvasItemsFromBlastSnapshot(snapshot.Blast)
	}
	return nil
}

func canvasItemFromKeywordSnapshot(module *sessionsnapshot.KeywordResultV2) model.CanvasItem {
	rows := flattenKeywordSearchGroups(module.Groups)
	itemRows := make([]model.CanvasRow, 0, len(rows))
	selected := make([]bool, 0, len(rows))
	for _, row := range rows {
		copyRow := row
		itemRows = append(itemRows, model.CanvasRow{Kind: model.CanvasKindKeyword, KeywordRow: &copyRow})
		selected = append(selected, true)
	}
	title := strings.TrimSpace(module.SelectedSpecies.DisplayLabel())
	if title == "" {
		title = "keyword"
	}
	return model.CanvasItem{
		Title:        title,
		Subtitle:     fmt.Sprintf("%d/%d lines", len(itemRows), len(itemRows)),
		Kind:         model.CanvasKindKeyword,
		Rows:         assignCanvasRowNumbers(itemRows, 1),
		Selected:     selected,
		SourceLabel:  title,
		ImportedFrom: "snapshot",
	}
}

func canvasItemsFromBlastSnapshot(module *sessionsnapshot.BlastResultV2) []model.CanvasItem {
	items := make([]model.CanvasItem, 0, len(module.Runs))
	for _, run := range module.Runs {
		if len(run.Results.Rows) == 0 {
			continue
		}
		rows := make([]model.CanvasRow, 0, len(run.Results.Rows))
		selected := make([]bool, 0, len(run.Results.Rows))
		for _, row := range run.Results.Rows {
			copyRow := row
			rows = append(rows, model.CanvasRow{Kind: model.CanvasKindBlast, BlastRow: &copyRow})
			selected = append(selected, true)
		}
		items = append(items, model.CanvasItem{
			Title:        strings.TrimSpace(firstNonEmpty(run.Item.LabelName, run.Item.RawInput, fmt.Sprintf("%d", len(items)+1))),
			Subtitle:     fmt.Sprintf("%d/%d lines", len(rows), len(rows)),
			Kind:         model.CanvasKindBlast,
			Rows:         assignCanvasRowNumbers(rows, 1),
			Selected:     selected,
			SourceLabel:  strings.TrimSpace(firstNonEmpty(run.Item.LabelName, run.Item.RawInput)),
			ImportedFrom: "snapshot",
		})
	}
	return items
}

func canvasItemFromBlastRows(title string, sourceLabel string, rows []model.BlastResultRow, selectedMask []bool) model.CanvasItem {
	itemRows := make([]model.CanvasRow, 0, len(rows))
	mask := selectedMask
	if len(mask) != len(rows) {
		mask = make([]bool, len(rows))
		for i := range mask {
			mask[i] = true
		}
	}
	for _, row := range rows {
		copyRow := row
		itemRows = append(itemRows, model.CanvasRow{Kind: model.CanvasKindBlast, BlastRow: &copyRow})
	}
	title = strings.TrimSpace(title)
	if title == "" {
		title = strings.TrimSpace(sourceLabel)
	}
	if title == "" {
		title = "1"
	}
	return model.CanvasItem{
		Title:       title,
		Subtitle:    fmt.Sprintf("%d/%d lines", len(itemRows), countTrueBools(mask)),
		Kind:        model.CanvasKindBlast,
		Rows:        assignCanvasRowNumbers(itemRows, 1),
		Selected:    append([]bool(nil), mask...),
		SourceLabel: strings.TrimSpace(sourceLabel),
	}
}

func canvasItemsFromKeywordSelection(groups []model.KeywordSearchGroup, selectedRows []model.KeywordResultRow) []model.CanvasItem {
	if len(selectedRows) == 0 {
		return nil
	}
	selectedKeys := make(map[string]int, len(selectedRows))
	for i, row := range selectedRows {
		selectedKeys[canvasKeywordRowKey(row, i)]++
	}
	items := make([]model.CanvasItem, 0, len(groups))
	for _, group := range groups {
		itemRows := make([]model.CanvasRow, 0, len(group.Rows))
		selectedMask := make([]bool, 0, len(group.Rows))
		for rowIndex, row := range group.Rows {
			key := canvasKeywordRowKey(row, rowIndex)
			if selectedKeys[key] <= 0 {
				continue
			}
			selectedKeys[key]--
			copyRow := row
			itemRows = append(itemRows, model.CanvasRow{Kind: model.CanvasKindKeyword, KeywordRow: &copyRow})
			selectedMask = append(selectedMask, true)
		}
		if len(itemRows) == 0 {
			continue
		}
		title := strings.TrimSpace(group.SearchTerm)
		if title == "" {
			title = strings.TrimSpace(group.LabelName)
		}
		if title == "" {
			title = "1"
		}
		items = append(items, model.CanvasItem{
			Title:       title,
			Subtitle:    fmt.Sprintf("%d/%d lines", len(itemRows), len(itemRows)),
			Kind:        model.CanvasKindKeyword,
			Rows:        assignCanvasRowNumbers(itemRows, 1),
			Selected:    selectedMask,
			SourceLabel: strings.TrimSpace(group.LabelName),
		})
	}
	if len(items) == 0 {
		itemRows := make([]model.CanvasRow, 0, len(selectedRows))
		selectedMask := make([]bool, 0, len(selectedRows))
		for _, row := range selectedRows {
			copyRow := row
			itemRows = append(itemRows, model.CanvasRow{Kind: model.CanvasKindKeyword, KeywordRow: &copyRow})
			selectedMask = append(selectedMask, true)
		}
		items = append(items, model.CanvasItem{
			Title:       "1",
			Subtitle:    fmt.Sprintf("%d/%d lines", len(itemRows), len(itemRows)),
			Kind:        model.CanvasKindKeyword,
			Rows:        assignCanvasRowNumbers(itemRows, 1),
			Selected:    selectedMask,
			SourceLabel: strings.TrimSpace(firstNonEmpty(selectedRows[0].LabelName, selectedRows[0].SearchTerm)),
		})
	}
	return items
}

func canvasItemsFromBlastRuns(runs []blastQueryRun, selectedByRun [][]bool) []model.CanvasItem {
	items := make([]model.CanvasItem, 0, len(runs))
	for runIndex, run := range runs {
		mask := []bool(nil)
		if runIndex < len(selectedByRun) {
			mask = selectedByRun[runIndex]
		}
		selectedRows := make([]model.BlastResultRow, 0, len(run.Results.Rows))
		selectedMask := make([]bool, 0, len(run.Results.Rows))
		for rowIndex, row := range run.Results.Rows {
			if len(mask) > 0 {
				if rowIndex >= len(mask) || !mask[rowIndex] {
					continue
				}
			}
			selectedRows = append(selectedRows, row)
			selectedMask = append(selectedMask, true)
		}
		if len(selectedRows) == 0 {
			continue
		}
		title := strings.TrimSpace(firstNonEmpty(run.Item.LabelName, run.Item.RawInput))
		sourceLabel := strings.TrimSpace(firstNonEmpty(run.Item.LabelName, run.Item.RawInput))
		items = append(items, canvasItemFromBlastRows(title, sourceLabel, selectedRows, selectedMask))
	}
	return items
}

func canvasKeywordRowKey(row model.KeywordResultRow, index int) string {
	return strings.Join([]string{
		strings.TrimSpace(row.SourceDatabase),
		strings.TrimSpace(row.SearchTerm),
		strings.TrimSpace(row.SequenceID),
		strings.TrimSpace(row.ProteinID),
		strings.TrimSpace(row.TranscriptID),
		strings.TrimSpace(row.GeneIdentifier),
		fmt.Sprintf("%d", index),
	}, "|")
}

func countTrueBools(values []bool) int {
	count := 0
	for _, value := range values {
		if value {
			count++
		}
	}
	return count
}

func (w *BlastWizard) runKeywordRowsCanvasMode(ctx context.Context, selected model.SpeciesCandidate, groups []model.KeywordSearchGroup, rows []model.KeywordResultRow) error {
	_ = selected
	items := canvasItemsFromKeywordSelection(groups, rows)
	if len(items) == 0 {
		return nil
	}
	return w.openCanvasChildOrInline(ctx, items, 0, nextCanvasNumericID(items))
}

func (w *BlastWizard) runBlastRowsCanvasMode(ctx context.Context, title string, sourceLabel string, rows []model.BlastResultRow) error {
	if len(rows) == 0 {
		return nil
	}
	item := canvasItemFromBlastRows("1", sourceLabel, rows, nil)
	return w.openCanvasChildOrInline(ctx, []model.CanvasItem{item}, 0, nextCanvasNumericID([]model.CanvasItem{item}))
}

func (w *BlastWizard) runBlastRunsCanvasMode(ctx context.Context, runs []blastQueryRun, selectedByRun [][]bool) error {
	items := canvasItemsFromBlastRuns(runs, selectedByRun)
	if len(items) == 0 {
		return nil
	}
	return w.openCanvasChildOrInline(ctx, items, 0, nextCanvasNumericID(items))
}

func (w *BlastWizard) openCanvasChildOrInline(ctx context.Context, items []model.CanvasItem, currentItem int, nextNumericID int) error {
	if w.shouldSpawnChildTab() {
		handoff := w.SnapshotHandoff("", ModeCanvas, "", w.instanceID, w.instanceRunID)
		handoff.StartupSource = "canvas-transfer"
		handoff.BlastContext.PendingMode = string(ModeCanvas)
		handoff.BlastContext.TransferKind = "canvas_items"
		handoff.BlastContext.TransferCanvasItems = cloneCanvasItems(items)
		handoff.BlastContext.TransferCanvasCurrent = currentItem
		handoff.BlastContext.TransferCanvasNextID = nextNumericID
		return w.spawnChildTab(ctx, "", ModeCanvas, handoff)
	}
	return w.runCanvasMode(ctx, canvasLaunchState{
		Items:         cloneCanvasItems(items),
		CurrentItem:   currentItem,
		NextNumericID: nextNumericID,
		SaveBaseName:  canvasDefaultSaveName(items),
	})
}

func cloneCanvasItems(items []model.CanvasItem) []model.CanvasItem {
	out := make([]model.CanvasItem, len(items))
	for i := range items {
		out[i] = model.CanvasItem{
			Title:        items[i].Title,
			Subtitle:     items[i].Subtitle,
			Kind:         items[i].Kind,
			Selected:     append([]bool(nil), items[i].Selected...),
			SourceLabel:  items[i].SourceLabel,
			ImportedFrom: items[i].ImportedFrom,
			Rows:         make([]model.CanvasRow, len(items[i].Rows)),
		}
		for j := range items[i].Rows {
			row := items[i].Rows[j]
			out[i].Rows[j].RowNumber = row.RowNumber
			out[i].Rows[j].Kind = row.Kind
			if row.KeywordRow != nil {
				copyRow := *row.KeywordRow
				if row.KeywordRow.ExtraColumns != nil {
					copyRow.ExtraColumns = make(map[string]string, len(row.KeywordRow.ExtraColumns))
					for key, value := range row.KeywordRow.ExtraColumns {
						copyRow.ExtraColumns[key] = value
					}
				}
				out[i].Rows[j].KeywordRow = &copyRow
			}
			if row.BlastRow != nil {
				copyRow := *row.BlastRow
				out[i].Rows[j].BlastRow = &copyRow
			}
			if row.FASTA != nil {
				copySource := *row.FASTA
				out[i].Rows[j].FASTA = &copySource
			}
		}
	}
	return out
}
