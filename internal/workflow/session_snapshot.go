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
	"strconv"
	"strings"
	"time"

	"github.com/KiriKirby/phytozome-go/internal/appfs"
	"github.com/KiriKirby/phytozome-go/internal/model"
	"github.com/KiriKirby/phytozome-go/internal/prompt"
	"github.com/KiriKirby/phytozome-go/internal/sessionsnapshot"
	"github.com/KiriKirby/phytozome-go/internal/tui"
)

type sessionSnapshotLoadResult struct {
	snapshot   sessionsnapshot.Snapshot
	restoreErr error
}

func (w *BlastWizard) openSessionSnapshotTool(ctx context.Context) error {
	for {
		input, err := w.prompt.OpenSessionInput()
		if err != nil {
			return err
		}
		outputDir, err := appfs.OutputDir()
		if err != nil {
			return err
		}
		path, err := sessionsnapshot.ResolveOpenPath(input, outputDir)
		if err != nil {
			if infoErr := w.showInfo("Open session", err.Error(), nil); infoErr != nil {
				return infoErr
			}
			continue
		}
		load, err := w.loadSessionSnapshotWithProgress(ctx, path)
		if err != nil {
			if infoErr := w.showInfo("Open session", fmt.Sprintf("Could not open session snapshot.\n\n%s\n\n%v", path, err), nil); infoErr != nil {
				return infoErr
			}
			continue
		}
		snapshot := load.snapshot
		database := normalizeSnapshotDatabase(snapshot.Context.Database)
		if database == "" {
			database = inferSnapshotDatabase(snapshot)
		}
		if database != "" {
			src, srcErr := w.dataSourceForDatabase(database)
			if srcErr != nil {
				if infoErr := w.showInfo("Open session", fmt.Sprintf("The snapshot database %q is not available.\n\n%v", database, srcErr), nil); infoErr != nil {
					return infoErr
				}
				continue
			}
			w.source = src
			w.prompt.SetDatabaseContext(databaseDisplayName(src.Name()))
		}
		restoreErr := load.restoreErr
		if restoreErr != nil {
			message := fmt.Sprintf("The session snapshot opened, but some frozen files could not be restored back to disk.\n\nThe program can still continue and will fall back only when those files are needed.\n\n%v", restoreErr)
			if infoErr := w.showInfo("Open session", message, nil); infoErr != nil {
				return infoErr
			}
		}
		switch {
		case snapshot.Keyword != nil:
			return w.reviewKeywordSnapshot(ctx, snapshot)
		case snapshot.Blast != nil:
			return w.reviewBlastSnapshot(ctx, snapshot)
		case snapshot.Canvas != nil:
			return w.reviewCanvasSnapshot(ctx, snapshot)
		default:
			if infoErr := w.showInfo("Open session", "Session snapshot contains no supported result module.", nil); infoErr != nil {
				return infoErr
			}
		}
	}
}

func (w *BlastWizard) loadSessionSnapshotWithProgress(ctx context.Context, path string) (sessionSnapshotLoadResult, error) {
	if w.suppressTaskModals {
		snapshot, err := sessionsnapshot.ReadFile(path)
		if err != nil {
			return sessionSnapshotLoadResult{}, err
		}
		return sessionSnapshotLoadResult{
			snapshot:   snapshot,
			restoreErr: hydrateSnapshotArtifacts(snapshot),
		}, nil
	}
	return tui.RunProgressTaskValueContext(tui.TaskPage{
		Path:        w.tuiPath("Startup", "Explore", "Open session"),
		Title:       "Opening session snapshot",
		Description: "Reading the saved workflow state and restoring recorded files before the review screen opens.",
		Initial:     "Opening session snapshot...",
		Total:       2,
		CancelError: prompt.ErrBackToDatabaseSelection,
	}, func(taskCtx context.Context, update func(int, string)) (sessionSnapshotLoadResult, error) {
		progress := safeProgress(update)
		progress(0, "Reading session snapshot from disk...")
		snapshot, err := sessionsnapshot.ReadFile(path)
		if err != nil {
			return sessionSnapshotLoadResult{}, err
		}
		progress(1, "Restoring recorded files to disk...")
		restoreErr := hydrateSnapshotArtifacts(snapshot)
		progress(2, "Session snapshot loaded.")
		return sessionSnapshotLoadResult{snapshot: snapshot, restoreErr: restoreErr}, nil
	})
}

func (w *BlastWizard) writeKeywordSessionSnapshot(selected model.SpeciesCandidate, groups []model.KeywordSearchGroup, selectedMask []bool, mode QueryMode, settings exportSettings, reportCtx *keywordReportRunContext) (string, error) {
	path := sessionsnapshot.DefaultFilePath(settings.OutputDir, settings.BaseName)
	snapshotContext := w.snapshotContext(string(mode), "keyword-result", "Keyword results")
	report := sessionsnapshot.ReportContextV1{}
	if reportCtx != nil {
		report = sessionsnapshot.ReportContextV1{
			QueryStarted:  reportCtx.QueryStarted,
			SearchEnded:   reportCtx.SearchEnded,
			ReviewStarted: reportCtx.ReviewStarted,
			LabelMode:     reportCtx.LabelMode,
		}
	}
	return path, w.writeSessionSnapshotWithProgress(path, "Writing keyword session snapshot...", func(ctx context.Context, update func(int, string)) (sessionsnapshot.Snapshot, error) {
		progress := safeProgress(update)
		progress(0, "Collecting keyword sequences...")
		sequenceCache, err := w.snapshotKeywordSequenceCacheWithModal(ctx, selected, flattenKeywordSearchGroups(groups))
		if err != nil {
			return sessionsnapshot.Snapshot{}, err
		}
		progress(1, "Preparing keyword snapshot data...")
		artifacts, artifactPayloads, err := snapshotArtifactBundle(path, settings.OutputDir)
		if err != nil {
			return sessionsnapshot.Snapshot{}, err
		}
		progress(2, "Keyword snapshot data prepared.")
		return sessionsnapshot.Snapshot{
			Context: snapshotContext,
			Keyword: &sessionsnapshot.KeywordResultV1{
				SelectedSpecies: selected,
				Groups:          cloneKeywordSearchGroups(groups),
				Selected:        append([]bool(nil), selectedMask...),
				ReportContext:   report,
			},
			KeywordReview: &sessionsnapshot.KeywordReviewStateV1{
				SelectionState: w.prompt.SnapshotKeywordReviewState(groups),
			},
			SequenceCache:    sequenceCache,
			ExportSettings:   snapshotExportSettings(w.prompt.SnapshotExportSettings(), settings),
			Handoff:          w.snapshotHandoffState(),
			Artifacts:        artifacts,
			RuntimeCache:     w.snapshotRuntimeCache(),
			ArtifactPayloads: artifactPayloads,
		}, nil
	})
}

func (w *BlastWizard) writeBlastSessionSnapshot(selected model.SpeciesCandidate, prepared []blastQueryItem, runs []blastQueryRun, configuredRequest model.BlastRequest, originalRunCount int, selectedMask []bool, selectedByRun [][]bool, filterFlags []bool, filterFlagsByRun [][]bool, filterSettings model.BlastFilterSettings, filterApplied bool, filterCleared bool, settings exportSettings) (string, error) {
	path := sessionsnapshot.DefaultFilePath(settings.OutputDir, firstNonEmpty(settings.BaseName, filepath.Base(settings.OutputDir)))
	return path, w.writeSessionSnapshotWithProgress(path, "Writing BLAST session snapshot...", func(ctx context.Context, update func(int, string)) (sessionsnapshot.Snapshot, error) {
		progress := safeProgress(update)
		progress(0, "Collecting BLAST sequences...")
		sequenceCache, err := w.snapshotBlastSequenceCacheWithModal(ctx, runs)
		if err != nil {
			return sessionsnapshot.Snapshot{}, err
		}
		progress(1, "Preparing BLAST snapshot data...")
		review := &sessionsnapshot.BlastReviewStateV1{}
		if len(runs) <= 1 {
			var rows []model.BlastResultRow
			if len(runs) == 1 {
				rows = runs[0].Results.Rows
			}
			review.SingleSelectionState = w.prompt.SnapshotBlastRowReviewState(rows)
		} else {
			review.MultiSelectionState = w.prompt.SnapshotBlastRunsReviewState(blastRunViews(runs))
		}
		artifacts, artifactPayloads, err := snapshotArtifactBundle(path, settings.OutputDir)
		if err != nil {
			return sessionsnapshot.Snapshot{}, err
		}
		progress(2, "BLAST snapshot data prepared.")
		return sessionsnapshot.Snapshot{
			Context: w.snapshotContext(string(ModeBlast), "blast-result", "BLAST results"),
			Blast: &sessionsnapshot.BlastResultV1{
				SelectedSpecies:   selected,
				Prepared:          snapshotQueryItems(prepared),
				Runs:              snapshotBlastRuns(runs),
				ConfiguredRequest: configuredRequest,
				OriginalRunCount:  maxInt(1, originalRunCount),
				Selected:          append([]bool(nil), selectedMask...),
				SelectedByRun:     cloneBoolMatrixWorkflow(selectedByRun),
				FilterFlags:       append([]bool(nil), filterFlags...),
				FilterFlagsByRun:  cloneBoolMatrixWorkflow(filterFlagsByRun),
				FilterSettings:    filterSettings,
				FilterApplied:     filterApplied,
				FilterCleared:     filterCleared,
			},
			BlastReview:        review,
			SequenceCache:      sequenceCache,
			ExportSettings:     snapshotExportSettings(w.prompt.SnapshotExportSettings(), settings),
			ExternalReferences: snapshotExternalReferences(w.lastExternalRefs),
			Handoff:            w.snapshotHandoffState(),
			Artifacts:          artifacts,
			RuntimeCache:       w.snapshotRuntimeCache(),
			ArtifactPayloads:   artifactPayloads,
		}, nil
	})
}

func (w *BlastWizard) writeSessionSnapshotWithProgress(path string, initial string, build func(context.Context, func(int, string)) (sessionsnapshot.Snapshot, error)) error {
	if w.suppressTaskModals {
		snapshot, err := build(context.Background(), nil)
		if err != nil {
			return err
		}
		return sessionsnapshot.WriteFile(path, snapshot)
	}
	_, err := tui.RunProgressTaskValueContext(tui.TaskPage{
		Path:        w.tuiPath("Export", "Session snapshot"),
		Title:       "Writing session snapshot",
		Description: "Saving the frozen workflow state before the export finishes.",
		Initial:     initial,
		Total:       3,
		CancelError: prompt.ErrBackToRowSelection,
	}, func(taskCtx context.Context, update func(int, string)) (struct{}, error) {
		progress := safeProgress(update)
		progress(0, initial)
		snapshot, err := build(mergeContexts(context.Background(), taskCtx), update)
		if err != nil {
			return struct{}{}, err
		}
		progress(2, "Writing session snapshot to disk...")
		if err := sessionsnapshot.WriteFile(path, snapshot); err != nil {
			return struct{}{}, err
		}
		progress(3, "Session snapshot saved.")
		return struct{}{}, nil
	})
	return err
}

func (w *BlastWizard) snapshotContext(mode string, resultKind string, title string) sessionsnapshot.ContextV1 {
	database := ""
	if w.source != nil {
		database = strings.TrimSpace(w.source.Name())
	}
	return sessionsnapshot.ContextV1{
		CreatedAt:          time.Now(),
		ApplicationName:    firstNonEmpty(w.tuiInfo.DisplayName, "phytozome GO"),
		ApplicationVersion: firstNonEmpty(w.tuiInfo.Version, "dev"),
		FormatName:         sessionsnapshot.FormatName,
		FormatVersion:      sessionsnapshot.FormatVersion,
		Database:           database,
		Mode:               strings.TrimSpace(mode),
		ResultKind:         strings.TrimSpace(resultKind),
		Title:              strings.TrimSpace(title),
	}
}

func (w *BlastWizard) reviewKeywordSnapshot(ctx context.Context, snapshot sessionsnapshot.Snapshot) error {
	selected, groups, reportCtx, module, err := w.hydrateKeywordSnapshot(snapshot)
	if err != nil {
		return err
	}

	initial := append([]bool(nil), module.Selected...)
	for {
		selection, err := w.prompt.SelectKeywordRowsWithInitial(groups, initial)
		if err != nil {
			if errors.Is(err, prompt.ErrBackToRowSelection) {
				continue
			}
			return err
		}
		initial = append([]bool(nil), selection.Selected...)
		if selection.RunBlast {
			if err := w.runKeywordBlastMode(ctx, selected, groups, selection.Rows, reportCtx); err != nil {
				if errors.Is(err, prompt.ErrBackToRowSelection) {
					continue
				}
				return err
			}
			continue
		}
		if !selection.GenerateFile {
			continue
		}
		if err := w.prepareAndExportKeywordSelectionWithMask(ctx, selected, groups, selection.Rows, selection.Selected, QueryMode(firstNonEmpty(snapshot.Context.Mode, string(ModeKeyword))), reportCtx); err != nil {
			if errors.Is(err, prompt.ErrBackToRowSelection) {
				continue
			}
			return err
		}
	}
}

func (w *BlastWizard) reviewBlastSnapshot(ctx context.Context, snapshot sessionsnapshot.Snapshot) error {
	selected, prepared, runs, module, err := w.hydrateBlastSnapshot(snapshot)
	if err != nil {
		return err
	}
	originalRunCount := maxInt(1, module.OriginalRunCount)
	if originalRunCount <= 1 && len(runs) > 1 {
		originalRunCount = len(runs)
	}
	if len(runs) == 0 {
		return fmt.Errorf("BLAST snapshot contains no runs")
	}
	if useSingleBlastRunReview(originalRunCount, runs) {
		return w.reviewSingleBlastSnapshot(ctx, selected, prepared, runs[0], module)
	}
	return w.reviewMultiBlastSnapshot(ctx, selected, prepared, runs, module)
}

func (w *BlastWizard) hydrateKeywordSnapshot(snapshot sessionsnapshot.Snapshot) (model.SpeciesCandidate, []model.KeywordSearchGroup, *keywordReportRunContext, *sessionsnapshot.KeywordResultV1, error) {
	module := snapshot.Keyword
	if module == nil {
		return model.SpeciesCandidate{}, nil, nil, nil, fmt.Errorf("missing keyword snapshot module")
	}
	selected := module.SelectedSpecies
	groups := cloneKeywordSearchGroups(module.Groups)
	w.hydrateSnapshotSequenceCache(snapshot.SequenceCache)
	w.hydrateRuntimeCache(snapshot.RuntimeCache)
	keywordReviewState := sessionsnapshot.KeywordReviewStateV1{}
	if snapshot.KeywordReview != nil {
		keywordReviewState = *snapshot.KeywordReview
	}
	w.prompt.RestoreKeywordReviewState(groups, module.Selected, keywordReviewState.SelectionState)
	w.hydrateCommonSnapshotState(snapshot)
	reportCtx := &keywordReportRunContext{
		Selected:      selected,
		QueryStarted:  module.ReportContext.QueryStarted,
		SearchEnded:   module.ReportContext.SearchEnded,
		ReviewStarted: module.ReportContext.ReviewStarted,
		LabelMode:     module.ReportContext.LabelMode,
	}
	w.lastKeywordSpecies = selected
	w.lastKeywordGroups = cloneKeywordSearchGroups(groups)
	w.lastKeywordReport = reportCtx
	w.postRunBackTarget = prompt.ErrBackToDatabaseSelection
	return selected, groups, reportCtx, module, nil
}

func (w *BlastWizard) hydrateBlastSnapshot(snapshot sessionsnapshot.Snapshot) (model.SpeciesCandidate, []blastQueryItem, []blastQueryRun, *sessionsnapshot.BlastResultV1, error) {
	module := snapshot.Blast
	if module == nil {
		return model.SpeciesCandidate{}, nil, nil, nil, fmt.Errorf("missing BLAST snapshot module")
	}
	w.hydrateSnapshotSequenceCache(snapshot.SequenceCache)
	w.hydrateRuntimeCache(snapshot.RuntimeCache)
	selected := module.SelectedSpecies
	prepared := blastQueryItemsFromSnapshot(module.Prepared)
	runs := blastRunsFromSnapshot(module.Runs)
	if len(prepared) == 0 {
		for _, run := range runs {
			prepared = append(prepared, run.Item)
		}
	}
	originalRunCount := module.OriginalRunCount
	if originalRunCount <= 0 {
		originalRunCount = len(runs)
	}
	blastReviewState := sessionsnapshot.BlastReviewStateV1{}
	if snapshot.BlastReview != nil {
		blastReviewState = *snapshot.BlastReview
	}
	if useSingleBlastRunReview(originalRunCount, runs) {
		var rows []model.BlastResultRow
		if len(runs) > 0 {
			rows = runs[0].Results.Rows
		}
		w.prompt.RestoreBlastRowReviewState(rows, module.Selected, module.FilterFlags, module.FilterSettings, blastReviewState.SingleSelectionState)
	} else {
		w.prompt.RestoreBlastRunsReviewState(blastRunViews(runs), module.SelectedByRun, module.FilterFlagsByRun, module.FilterSettings, blastReviewState.MultiSelectionState)
	}
	w.hydrateCommonSnapshotState(snapshot)
	w.lastBlastItems = cloneBlastQueryItems(prepared)
	w.lastBlastReviewContext = &blastReviewContext{
		Selected:          selected,
		Prepared:          cloneBlastQueryItems(prepared),
		OriginalPrepared:  cloneBlastQueryItems(prepared),
		Runs:              cloneBlastQueryRuns(runs),
		OriginalRuns:      cloneBlastQueryRuns(runs),
		ConfiguredRequest: module.ConfiguredRequest,
		OriginalRunCount:  originalRunCount,
	}
	return selected, prepared, runs, module, nil
}

func (w *BlastWizard) reviewSingleBlastSnapshot(ctx context.Context, selected model.SpeciesCandidate, prepared []blastQueryItem, run blastQueryRun, module *sessionsnapshot.BlastResultV1) error {
	for {
		selection, err := w.prompt.SelectBlastRowsWithInitial(run.Results.Rows, prompt.ErrBackToDatabaseSelection, false, module.Selected, module.FilterFlags)
		if err != nil {
			if errors.Is(err, prompt.ErrBackToRowSelection) {
				continue
			}
			return err
		}
		module.Selected = append([]bool(nil), selection.Selected...)
		module.FilterFlags = append([]bool(nil), selection.FilterFlags...)
		if selection.RunBlast {
			if err := w.runBlastRowsBlastMode(ctx, selected, selection.Rows); err != nil {
				if errors.Is(err, prompt.ErrBackToRowSelection) {
					continue
				}
				return err
			}
			continue
		}
		if !selection.GenerateFile {
			continue
		}
		if len(selection.Rows) == 0 {
			if err := w.showInfo("BLAST export", "No rows selected for this query. Export will be skipped.", prompt.ErrBackToRowSelection); err != nil {
				return err
			}
			continue
		}
		if err := w.exportSingleBlastRun(ctx, selected, prepared, run, selection.Rows, run.Results.Rows, selection.RowNumbers, selection.FilterFlags, module.ConfiguredRequest, false, selection); err != nil {
			if errors.Is(err, prompt.ErrBackToRowSelection) {
				continue
			}
			return err
		}
	}
}

func (w *BlastWizard) reviewMultiBlastSnapshot(ctx context.Context, selected model.SpeciesCandidate, prepared []blastQueryItem, runs []blastQueryRun, module *sessionsnapshot.BlastResultV1) error {
	for {
		selection, err := w.prompt.SelectBlastRunsWithInitial(blastRunViews(runs), prompt.ErrBackToDatabaseSelection, module.SelectedByRun, module.FilterFlagsByRun)
		if err != nil {
			if errors.Is(err, prompt.ErrBackToRowSelection) {
				continue
			}
			return err
		}
		module.SelectedByRun = cloneBoolMatrixWorkflow(selection.SelectedByRun)
		module.FilterFlagsByRun = cloneBoolMatrixWorkflow(selection.FilterFlagsByRun)
		if selection.RunBlast {
			if err := w.runBlastRowsBlastMode(ctx, selected, selection.Rows); err != nil {
				if errors.Is(err, prompt.ErrBackToRowSelection) {
					continue
				}
				return err
			}
			continue
		}
		if selection.DoneAll {
			if err := w.exportAllBlastRuns(ctx, selected, prepared, runs, selection.RowsByRun, selection.RowNumbersByRun, selection.FilterFlagsByRun, selection.SelectedByRun, module.ConfiguredRequest, selection.FilterSettings, selection.FilterApplied, selection.FilterCleared); err != nil {
				if errors.Is(err, prompt.ErrBackToRowSelection) {
					continue
				}
				return err
			}
			continue
		}
		if !selection.GenerateFile {
			continue
		}
		if selection.RunIndex < 0 || selection.RunIndex >= len(runs) {
			continue
		}
		run := runs[selection.RunIndex]
		if err := w.exportSingleBlastRun(ctx, selected, prepared, run, selection.Rows, run.Results.Rows, selection.RowNumbers, selection.FilterFlags, module.ConfiguredRequest, true, selection); err != nil {
			if errors.Is(err, prompt.ErrBackToRowSelection) {
				continue
			}
			return err
		}
	}
}

func (w *BlastWizard) reviewCanvasSnapshot(ctx context.Context, snapshot sessionsnapshot.Snapshot) error {
	module := snapshot.Canvas
	if module == nil {
		return fmt.Errorf("missing canvas snapshot module")
	}
	items := canvasItemsFromSnapshot(module.Items)
	if snapshot.CanvasReview != nil {
		w.prompt.RestoreCanvasReviewState(canvasStateKey("canvas"), snapshot.CanvasReview.SelectionState)
	}
	w.hydrateCommonSnapshotState(snapshot)
	w.hydrateRuntimeCache(snapshot.RuntimeCache)
	w.hydrateSnapshotSequenceCache(snapshot.SequenceCache)
	state := canvasLaunchState{
		Items:         items,
		CurrentItem:   module.CurrentItem,
		NextNumericID: module.NextNumericID,
		ImportedFrom:  module.ImportedFrom,
		SaveBaseName:  canvasDefaultSaveName(items),
	}
	return w.runCanvasMode(ctx, state)
}

func canvasItemsFromSnapshot(items []sessionsnapshot.CanvasItemV1) []model.CanvasItem {
	out := make([]model.CanvasItem, len(items))
	for i := range items {
		out[i].Title = items[i].Title
		out[i].Subtitle = items[i].Subtitle
		out[i].Kind = items[i].Kind
		out[i].Selected = append([]bool(nil), items[i].Selected...)
		out[i].SourceLabel = items[i].SourceLabel
		out[i].ImportedFrom = items[i].ImportedFrom
		out[i].Rows = make([]model.CanvasRow, len(items[i].Rows))
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

func snapshotQueryItems(items []blastQueryItem) []sessionsnapshot.BlastQueryItemV1 {
	out := make([]sessionsnapshot.BlastQueryItemV1, len(items))
	for i, item := range items {
		out[i] = snapshotQueryItem(item)
	}
	return out
}

func snapshotExportSettings(promptSettings prompt.ExportSettings, settings exportSettings) *sessionsnapshot.ExportSettingsV2 {
	return &sessionsnapshot.ExportSettingsV2{
		BaseName:  settings.BaseName,
		OutputDir: settings.OutputDir,
		Prompt: sessionsnapshot.PromptExportSettingsV2{
			BaseName:              promptSettings.BaseName,
			FolderName:            promptSettings.FolderName,
			WriteReport:           promptSettings.WriteReport,
			WriteSession:          promptSettings.WriteSession,
			WriteText:             promptSettings.WriteText,
			WriteExcel:            promptSettings.WriteExcel,
			WriteRawExcel:         promptSettings.WriteRawExcel,
			FastaHeaderMode:       promptSettings.FastaHeaderMode,
			UsePhgoHeader:         promptSettings.UsePhgoHeader,
			PrependOnlyFirstQuery: promptSettings.PrependOnlyFirstQuery,
		},
	}
}

func snapshotExternalReferences(config externalReferenceConfig) *sessionsnapshot.ExternalReferenceSettingsV2 {
	return &sessionsnapshot.ExternalReferenceSettingsV2{
		AutoLabelBlastHits: config.AutoLabelBlastHits,
		UseUniProt:         config.UseUniProt,
		UseInterPro:        config.UseInterPro,
		InterProSettings:   config.InterProSettings,
	}
}

func snapshotArtifactBundle(snapshotPath string, outputDir string) (*sessionsnapshot.ArtifactManifestV2, map[string][]byte, error) {
	return nil, nil, nil
}

func samePath(left string, right string) bool {
	if left == "" || right == "" {
		return false
	}
	return strings.EqualFold(filepath.Clean(left), filepath.Clean(right))
}

func hydrateSnapshotArtifacts(snapshot sessionsnapshot.Snapshot) error {
	if snapshot.Artifacts == nil || len(snapshot.Artifacts.Entries) == 0 {
		return nil
	}
	failures := make([]string, 0)
	for _, artifact := range snapshot.Artifacts.Entries {
		restorePath := strings.TrimSpace(artifact.RestorePath)
		if restorePath == "" {
			restorePath = strings.TrimSpace(artifact.SourcePath)
		}
		if restorePath == "" {
			failures = append(failures, fmt.Sprintf("%s: missing restore path", firstNonEmpty(artifact.Path, artifact.ID)))
			continue
		}
		payload, ok := snapshot.ArtifactPayloads[strings.TrimSpace(artifact.Path)]
		if !ok {
			failures = append(failures, fmt.Sprintf("%s: missing packed payload", firstNonEmpty(artifact.Path, artifact.ID)))
			continue
		}
		if err := appfs.WriteFileAtomic(restorePath, payload, 0o644); err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", restorePath, err))
		}
	}
	if len(failures) == 0 {
		return nil
	}
	return fmt.Errorf("%s", strings.Join(failures, "\n"))
}

func (w *BlastWizard) snapshotHandoffState() *sessionsnapshot.HandoffStateV2 {
	handoff := &sessionsnapshot.HandoffStateV2{
		PendingMode:           string(w.pendingMode),
		TransferKind:          strings.TrimSpace(w.transferKind),
		TransferTargetDB:      strings.TrimSpace(w.transferTargetDatabase),
		BlastProgramPath:      strings.TrimSpace(w.blastProgramPath),
		ReuseLastBlastInput:   w.reuseLastBlastInput,
		ReuseLastBlastRows:    w.reuseLastBlastRows,
		ReuseLastKeywordRows:  w.reuseLastKeywordRows,
		RewindBlastToInput:    w.rewindBlastToInput,
		RewindKeywordToInput:  w.rewindKeywordToInput,
		TransferSourceSpecies: w.transferSourceSpecies,
		TransferKeywordRows:   append([]model.KeywordResultRow(nil), w.transferKeywordRows...),
		TransferBlastRows:     append([]model.BlastResultRow(nil), w.transferBlastRows...),
		TransferCanvasItems:   cloneCanvasItems(w.transferCanvasItems),
		TransferCanvasCurrent: w.transferCanvasCurrent,
		TransferCanvasNextID:  w.transferCanvasNextID,
		LastBlastItems:        snapshotQueryItems(w.lastBlastItems),
		LastKeywordGroups:     cloneKeywordSearchGroups(w.lastKeywordGroups),
		LastKeywordSpecies:    w.lastKeywordSpecies,
	}
	if w.lastKeywordReport != nil {
		handoff.LastKeywordReport = &sessionsnapshot.ReportContextV2{
			QueryStarted:  w.lastKeywordReport.QueryStarted,
			SearchEnded:   w.lastKeywordReport.SearchEnded,
			ReviewStarted: w.lastKeywordReport.ReviewStarted,
			LabelMode:     w.lastKeywordReport.LabelMode,
		}
	}
	if w.lastBlastRowContext != nil {
		handoff.LastBlastRowContext = &sessionsnapshot.BlastRowContextV2{
			Rows:             append([]model.BlastResultRow(nil), w.lastBlastRowContext.Rows...),
			AllRows:          append([]model.BlastResultRow(nil), w.lastBlastRowContext.AllRows...),
			Numbers:          append([]int(nil), w.lastBlastRowContext.Numbers...),
			Flags:            append([]bool(nil), w.lastBlastRowContext.Flags...),
			SelectedRowsMask: append([]bool(nil), w.lastBlastRowContext.SelectedRowsMask...),
			Item:             snapshotQueryItem(w.lastBlastRowContext.Item),
			Selected:         w.lastBlastRowContext.Selected,
			Request:          w.lastBlastRowContext.Request,
			Results:          w.lastBlastRowContext.Results,
			Index:            w.lastBlastRowContext.Index,
			FilterSettings:   w.lastBlastRowContext.FilterSettings,
			FilterApplied:    w.lastBlastRowContext.FilterApplied,
			FilterCleared:    w.lastBlastRowContext.FilterCleared,
			FamilySettings:   w.lastBlastRowContext.FamilySettings,
		}
	}
	if w.lastBlastReviewContext != nil {
		handoff.LastBlastReview = &sessionsnapshot.BlastReviewContextV2{
			Selected:          w.lastBlastReviewContext.Selected,
			Prepared:          snapshotQueryItems(w.lastBlastReviewContext.Prepared),
			OriginalPrepared:  snapshotQueryItems(w.lastBlastReviewContext.OriginalPrepared),
			Runs:              snapshotBlastRuns(w.lastBlastReviewContext.Runs),
			OriginalRuns:      snapshotBlastRuns(w.lastBlastReviewContext.OriginalRuns),
			ConfiguredRequest: w.lastBlastReviewContext.ConfiguredRequest,
			OriginalRunCount:  w.lastBlastReviewContext.OriginalRunCount,
		}
	}
	return handoff
}

func (w *BlastWizard) hydrateCommonSnapshotState(snapshot sessionsnapshot.Snapshot) {
	if snapshot.ExportSettings != nil {
		w.prompt.RestoreExportSettings(prompt.ExportSettings{
			BaseName:              snapshot.ExportSettings.Prompt.BaseName,
			FolderName:            snapshot.ExportSettings.Prompt.FolderName,
			WriteReport:           snapshot.ExportSettings.Prompt.WriteReport,
			WriteSession:          snapshot.ExportSettings.Prompt.WriteSession,
			WriteText:             snapshot.ExportSettings.Prompt.WriteText,
			WriteExcel:            snapshot.ExportSettings.Prompt.WriteExcel,
			WriteRawExcel:         snapshot.ExportSettings.Prompt.WriteRawExcel,
			FastaHeaderMode:       snapshot.ExportSettings.Prompt.FastaHeaderMode,
			UsePhgoHeader:         snapshot.ExportSettings.Prompt.UsePhgoHeader,
			PrependOnlyFirstQuery: snapshot.ExportSettings.Prompt.PrependOnlyFirstQuery,
		})
	}
	if snapshot.ExternalReferences != nil {
		w.lastExternalRefs = externalReferenceConfig{
			AutoLabelBlastHits: snapshot.ExternalReferences.AutoLabelBlastHits,
			UseUniProt:         snapshot.ExternalReferences.UseUniProt,
			UseInterPro:        snapshot.ExternalReferences.UseInterPro,
			InterProSettings:   snapshot.ExternalReferences.InterProSettings,
		}
		w.prompt.RestoreExternalReferenceSettings(prompt.ExternalReferenceSettings{
			AutoLabelBlastHits: snapshot.ExternalReferences.AutoLabelBlastHits,
			UseUniProt:         snapshot.ExternalReferences.UseUniProt,
			UseInterPro:        snapshot.ExternalReferences.UseInterPro,
			InterProSettings:   snapshot.ExternalReferences.InterProSettings,
		})
	}
	if snapshot.Handoff != nil {
		w.pendingMode = QueryMode(snapshot.Handoff.PendingMode)
		w.transferKind = snapshot.Handoff.TransferKind
		w.transferTargetDatabase = snapshot.Handoff.TransferTargetDB
		w.blastProgramPath = snapshot.Handoff.BlastProgramPath
		w.reuseLastBlastInput = snapshot.Handoff.ReuseLastBlastInput
		w.reuseLastBlastRows = snapshot.Handoff.ReuseLastBlastRows
		w.reuseLastKeywordRows = snapshot.Handoff.ReuseLastKeywordRows
		w.rewindBlastToInput = snapshot.Handoff.RewindBlastToInput
		w.rewindKeywordToInput = snapshot.Handoff.RewindKeywordToInput
		w.transferSourceSpecies = snapshot.Handoff.TransferSourceSpecies
		w.transferKeywordRows = append([]model.KeywordResultRow(nil), snapshot.Handoff.TransferKeywordRows...)
		w.transferBlastRows = append([]model.BlastResultRow(nil), snapshot.Handoff.TransferBlastRows...)
		w.transferCanvasItems = cloneCanvasItems(snapshot.Handoff.TransferCanvasItems)
		w.transferCanvasCurrent = snapshot.Handoff.TransferCanvasCurrent
		w.transferCanvasNextID = snapshot.Handoff.TransferCanvasNextID
		w.lastBlastItems = blastQueryItemsFromSnapshot(snapshot.Handoff.LastBlastItems)
		w.lastKeywordGroups = cloneKeywordSearchGroups(snapshot.Handoff.LastKeywordGroups)
		w.lastKeywordSpecies = snapshot.Handoff.LastKeywordSpecies
		if snapshot.Handoff.LastKeywordReport != nil {
			w.lastKeywordReport = &keywordReportRunContext{
				Selected:      snapshot.Handoff.LastKeywordSpecies,
				QueryStarted:  snapshot.Handoff.LastKeywordReport.QueryStarted,
				SearchEnded:   snapshot.Handoff.LastKeywordReport.SearchEnded,
				ReviewStarted: snapshot.Handoff.LastKeywordReport.ReviewStarted,
				LabelMode:     snapshot.Handoff.LastKeywordReport.LabelMode,
			}
		}
		if snapshot.Handoff.LastBlastRowContext != nil {
			w.lastBlastRowContext = &blastRowContext{
				Rows:             append([]model.BlastResultRow(nil), snapshot.Handoff.LastBlastRowContext.Rows...),
				AllRows:          append([]model.BlastResultRow(nil), snapshot.Handoff.LastBlastRowContext.AllRows...),
				Numbers:          append([]int(nil), snapshot.Handoff.LastBlastRowContext.Numbers...),
				Flags:            append([]bool(nil), snapshot.Handoff.LastBlastRowContext.Flags...),
				SelectedRowsMask: append([]bool(nil), snapshot.Handoff.LastBlastRowContext.SelectedRowsMask...),
				Item:             blastQueryItemFromSnapshot(snapshot.Handoff.LastBlastRowContext.Item),
				Selected:         snapshot.Handoff.LastBlastRowContext.Selected,
				Request:          snapshot.Handoff.LastBlastRowContext.Request,
				Results:          snapshot.Handoff.LastBlastRowContext.Results,
				Index:            snapshot.Handoff.LastBlastRowContext.Index,
				FilterSettings:   snapshot.Handoff.LastBlastRowContext.FilterSettings,
				FilterApplied:    snapshot.Handoff.LastBlastRowContext.FilterApplied,
				FilterCleared:    snapshot.Handoff.LastBlastRowContext.FilterCleared,
				FamilySettings:   snapshot.Handoff.LastBlastRowContext.FamilySettings,
			}
		}
		if snapshot.Handoff.LastBlastReview != nil {
			w.lastBlastReviewContext = &blastReviewContext{
				Selected:          snapshot.Handoff.LastBlastReview.Selected,
				Prepared:          blastQueryItemsFromSnapshot(snapshot.Handoff.LastBlastReview.Prepared),
				OriginalPrepared:  blastQueryItemsFromSnapshot(snapshot.Handoff.LastBlastReview.OriginalPrepared),
				Runs:              blastRunsFromSnapshot(snapshot.Handoff.LastBlastReview.Runs),
				OriginalRuns:      blastRunsFromSnapshot(snapshot.Handoff.LastBlastReview.OriginalRuns),
				ConfiguredRequest: snapshot.Handoff.LastBlastReview.ConfiguredRequest,
				OriginalRunCount:  snapshot.Handoff.LastBlastReview.OriginalRunCount,
			}
		}
	}
}

func (w *BlastWizard) snapshotRuntimeCache() *sessionsnapshot.RuntimeCacheV2 {
	cache := &sessionsnapshot.RuntimeCacheV2{}

	w.blastLabelLookupMu.Lock()
	for key, result := range w.blastLabelLookupCache {
		cache.BlastLabelLookups = append(cache.BlastLabelLookups, sessionsnapshot.BlastLabelLookupCacheEntryV2{
			Key:           key,
			Label:         result.Label,
			Aliases:       append([]string(nil), result.Aliases...),
			TaskTimestamp: result.TaskTimestamp,
			ItemIndex:     result.ItemIndex,
		})
	}
	w.blastLabelLookupMu.Unlock()

	w.blastHitLabelLookupMu.RLock()
	for key, result := range w.blastHitLabelLookupCache {
		cache.BlastHitLabelLookups = append(cache.BlastHitLabelLookups, sessionsnapshot.BlastHitLabelLookupCacheEntryV2{
			Key:       key,
			Label:     result.Label,
			LabelType: result.LabelType,
			Aliases:   append([]string(nil), result.Aliases...),
		})
	}
	w.blastHitLabelLookupMu.RUnlock()

	w.rowUniProtAccessionsMu.Lock()
	for key, known := range w.rowUniProtAccessionsKnown {
		cache.RowUniProtAccessions = append(cache.RowUniProtAccessions, sessionsnapshot.RowUniProtAccessionsCacheEntryV2{
			Key:        key,
			Known:      known,
			Accessions: append([]string(nil), w.rowUniProtAccessionsCache[key]...),
		})
	}
	w.rowUniProtAccessionsMu.Unlock()

	w.uniProtLookupMu.RLock()
	for key, result := range w.uniProtLookupCache {
		cache.UniProtLookups = append(cache.UniProtLookups, sessionsnapshot.UniProtLookupCacheEntryV2{
			Key:   key,
			Entry: result.entry,
			OK:    result.ok,
			Error: snapshotErrorString(result.err),
		})
	}
	w.uniProtLookupMu.RUnlock()

	w.interProLookupMu.RLock()
	for key, result := range w.interProLookupCache {
		cache.InterProLookups = append(cache.InterProLookups, sessionsnapshot.InterProLookupCacheEntryV2{
			Key:   key,
			Entry: result.entry,
			OK:    result.ok,
			Error: snapshotErrorString(result.err),
		})
	}
	w.interProLookupMu.RUnlock()

	w.keywordBlastItemMu.RLock()
	for key, item := range w.keywordBlastItemCache {
		cache.KeywordBlastItems = append(cache.KeywordBlastItems, sessionsnapshot.KeywordBlastItemCacheEntryV2{
			Key:  key,
			Item: snapshotQueryItem(item),
		})
	}
	w.keywordBlastItemMu.RUnlock()

	w.querySourceResolveMu.RLock()
	for key, source := range w.querySourceResolveCache {
		cache.QuerySourceResolutions = append(cache.QuerySourceResolutions, sessionsnapshot.QuerySourceResolutionCacheEntryV2{
			Key:    key,
			Source: source,
		})
	}
	w.querySourceResolveMu.RUnlock()

	w.keywordTermRowsMu.RLock()
	for key, rows := range w.keywordTermRowsCache {
		cache.KeywordTermRows = append(cache.KeywordTermRows, sessionsnapshot.KeywordTermRowsCacheEntryV2{
			Key:  key,
			Rows: cloneKeywordResultRows(rows),
		})
	}
	w.keywordTermRowsMu.RUnlock()

	w.proteinSequenceMu.RLock()
	for key, sequence := range w.proteinSequenceCache {
		cache.ProteinSequences = append(cache.ProteinSequences, sessionsnapshot.ProteinSequenceCacheEntryV2{
			Key:      key,
			Sequence: sequence,
		})
	}
	for key, err := range w.proteinSequenceMiss {
		cache.ProteinSequenceMisses = append(cache.ProteinSequenceMisses, sessionsnapshot.ProteinSequenceMissCacheEntryV2{
			Key:   key,
			Error: snapshotErrorString(err),
		})
	}
	w.proteinSequenceMu.RUnlock()

	w.speciesCandidatesMu.Lock()
	for key, candidates := range w.speciesCandidatesCache {
		cache.SpeciesCandidates = append(cache.SpeciesCandidates, sessionsnapshot.SpeciesCandidatesCacheEntryV2{
			Key:        key,
			Candidates: append([]model.SpeciesCandidate(nil), candidates...),
		})
	}
	w.speciesCandidatesMu.Unlock()

	if len(cache.BlastLabelLookups) == 0 &&
		len(cache.BlastHitLabelLookups) == 0 &&
		len(cache.RowUniProtAccessions) == 0 &&
		len(cache.UniProtLookups) == 0 &&
		len(cache.InterProLookups) == 0 &&
		len(cache.KeywordBlastItems) == 0 &&
		len(cache.QuerySourceResolutions) == 0 &&
		len(cache.KeywordTermRows) == 0 &&
		len(cache.ProteinSequences) == 0 &&
		len(cache.ProteinSequenceMisses) == 0 &&
		len(cache.SpeciesCandidates) == 0 {
		return nil
	}
	return cache
}

func (w *BlastWizard) hydrateRuntimeCache(cache *sessionsnapshot.RuntimeCacheV2) {
	if cache == nil {
		return
	}

	w.blastLabelLookupMu.Lock()
	if w.blastLabelLookupCache == nil {
		w.blastLabelLookupCache = make(map[string]blastAutoLabelResult)
	}
	for _, entry := range cache.BlastLabelLookups {
		w.blastLabelLookupCache[entry.Key] = blastAutoLabelResult{
			Label:         entry.Label,
			Aliases:       append([]string(nil), entry.Aliases...),
			TaskTimestamp: entry.TaskTimestamp,
			ItemIndex:     entry.ItemIndex,
		}
	}
	w.blastLabelLookupMu.Unlock()

	w.blastHitLabelLookupMu.Lock()
	if w.blastHitLabelLookupCache == nil {
		w.blastHitLabelLookupCache = make(map[string]blastHitLabelIdentification)
	}
	for _, entry := range cache.BlastHitLabelLookups {
		w.blastHitLabelLookupCache[entry.Key] = blastHitLabelIdentification{
			Label:     entry.Label,
			LabelType: entry.LabelType,
			Aliases:   append([]string(nil), entry.Aliases...),
		}
	}
	w.blastHitLabelLookupMu.Unlock()

	w.rowUniProtAccessionsMu.Lock()
	if w.rowUniProtAccessionsCache == nil {
		w.rowUniProtAccessionsCache = make(map[string][]string)
	}
	if w.rowUniProtAccessionsKnown == nil {
		w.rowUniProtAccessionsKnown = make(map[string]bool)
	}
	for _, entry := range cache.RowUniProtAccessions {
		w.rowUniProtAccessionsCache[entry.Key] = append([]string(nil), entry.Accessions...)
		w.rowUniProtAccessionsKnown[entry.Key] = entry.Known
	}
	w.rowUniProtAccessionsMu.Unlock()

	w.uniProtLookupMu.Lock()
	if w.uniProtLookupCache == nil {
		w.uniProtLookupCache = make(map[string]uniProtLookupResult)
	}
	for _, entry := range cache.UniProtLookups {
		w.uniProtLookupCache[entry.Key] = uniProtLookupResult{
			entry: entry.Entry,
			ok:    entry.OK,
			err:   snapshotErrorValue(entry.Error),
		}
	}
	w.uniProtLookupMu.Unlock()

	w.interProLookupMu.Lock()
	if w.interProLookupCache == nil {
		w.interProLookupCache = make(map[string]interProLookupResult)
	}
	for _, entry := range cache.InterProLookups {
		w.interProLookupCache[entry.Key] = interProLookupResult{
			entry: entry.Entry,
			ok:    entry.OK,
			err:   snapshotErrorValue(entry.Error),
		}
	}
	w.interProLookupMu.Unlock()

	w.keywordBlastItemMu.Lock()
	if w.keywordBlastItemCache == nil {
		w.keywordBlastItemCache = make(map[string]blastQueryItem)
	}
	for _, entry := range cache.KeywordBlastItems {
		w.keywordBlastItemCache[entry.Key] = blastQueryItemFromSnapshot(entry.Item)
	}
	w.keywordBlastItemMu.Unlock()

	w.querySourceResolveMu.Lock()
	if w.querySourceResolveCache == nil {
		w.querySourceResolveCache = make(map[string]model.QuerySequenceSource)
	}
	for _, entry := range cache.QuerySourceResolutions {
		w.querySourceResolveCache[entry.Key] = entry.Source
	}
	w.querySourceResolveMu.Unlock()

	w.keywordTermRowsMu.Lock()
	if w.keywordTermRowsCache == nil {
		w.keywordTermRowsCache = make(map[string][]model.KeywordResultRow)
	}
	for _, entry := range cache.KeywordTermRows {
		w.keywordTermRowsCache[entry.Key] = cloneKeywordResultRows(entry.Rows)
	}
	w.keywordTermRowsMu.Unlock()

	w.proteinSequenceMu.Lock()
	if w.proteinSequenceCache == nil {
		w.proteinSequenceCache = make(map[string]model.ProteinSequenceData)
	}
	if w.proteinSequenceMiss == nil {
		w.proteinSequenceMiss = make(map[string]error)
	}
	for _, entry := range cache.ProteinSequences {
		w.proteinSequenceCache[entry.Key] = entry.Sequence
	}
	for _, entry := range cache.ProteinSequenceMisses {
		w.proteinSequenceMiss[entry.Key] = snapshotErrorValue(entry.Error)
	}
	w.proteinSequenceMu.Unlock()

	w.speciesCandidatesMu.Lock()
	if w.speciesCandidatesCache == nil {
		w.speciesCandidatesCache = make(map[string][]model.SpeciesCandidate)
	}
	for _, entry := range cache.SpeciesCandidates {
		w.speciesCandidatesCache[entry.Key] = append([]model.SpeciesCandidate(nil), entry.Candidates...)
	}
	w.speciesCandidatesMu.Unlock()
}

func snapshotErrorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func snapshotErrorValue(message string) error {
	message = strings.TrimSpace(message)
	if message == "" {
		return nil
	}
	return fmt.Errorf("%s", message)
}

func snapshotQueryItem(item blastQueryItem) sessionsnapshot.BlastQueryItemV1 {
	return sessionsnapshot.BlastQueryItemV1{
		RawInput:            item.RawInput,
		LabelName:           item.LabelName,
		Sequence:            item.Sequence,
		ProteinSequence:     item.ProteinSequence,
		NucleotideSequence:  item.NucleotideSequence,
		QuerySource:         cloneQuerySource(item.QuerySource),
		FromKeyword:         item.FromKeyword,
		FamilyName:          item.FamilyName,
		MemberLabel:         item.MemberLabel,
		FamilyGroupSource:   item.FamilyGroupSource,
		FamilyDetectionRule: item.FamilyDetectionRule,
		FamilySources:       cloneQuerySources(item.FamilySources),
		FamilySettings:      item.FamilySettings,
	}
}

func snapshotBlastRuns(runs []blastQueryRun) []sessionsnapshot.BlastRunV1 {
	out := make([]sessionsnapshot.BlastRunV1, len(runs))
	for i, run := range runs {
		out[i] = sessionsnapshot.BlastRunV1{
			Index:           run.Index,
			Item:            snapshotQueryItem(run.Item),
			Request:         run.Request,
			Results:         run.Results,
			RowsBeforeMerge: run.RowsBeforeMerge,
			RowsAfterMerge:  run.RowsAfterMerge,
		}
	}
	return out
}

func blastQueryItemsFromSnapshot(items []sessionsnapshot.BlastQueryItemV1) []blastQueryItem {
	out := make([]blastQueryItem, len(items))
	for i, item := range items {
		out[i] = blastQueryItemFromSnapshot(item)
	}
	return out
}

func blastQueryItemFromSnapshot(item sessionsnapshot.BlastQueryItemV1) blastQueryItem {
	return blastQueryItem{
		RawInput:            item.RawInput,
		LabelName:           item.LabelName,
		Sequence:            item.Sequence,
		ProteinSequence:     item.ProteinSequence,
		NucleotideSequence:  item.NucleotideSequence,
		QuerySource:         cloneQuerySource(item.QuerySource),
		FromKeyword:         item.FromKeyword,
		FamilyName:          item.FamilyName,
		MemberLabel:         item.MemberLabel,
		FamilyGroupSource:   item.FamilyGroupSource,
		FamilyDetectionRule: item.FamilyDetectionRule,
		FamilySources:       cloneQuerySources(item.FamilySources),
		FamilySettings:      item.FamilySettings,
	}
}

func blastRunsFromSnapshot(runs []sessionsnapshot.BlastRunV1) []blastQueryRun {
	out := make([]blastQueryRun, len(runs))
	for i, run := range runs {
		out[i] = blastQueryRun{
			Index:           run.Index,
			Item:            blastQueryItemFromSnapshot(run.Item),
			Request:         run.Request,
			Results:         run.Results,
			RowsBeforeMerge: run.RowsBeforeMerge,
			RowsAfterMerge:  run.RowsAfterMerge,
		}
	}
	return out
}

func cloneQuerySource(source *model.QuerySequenceSource) *model.QuerySequenceSource {
	if source == nil {
		return nil
	}
	copy := *source
	return &copy
}

func cloneQuerySources(sources []*model.QuerySequenceSource) []*model.QuerySequenceSource {
	out := make([]*model.QuerySequenceSource, 0, len(sources))
	for _, source := range sources {
		out = append(out, cloneQuerySource(source))
	}
	return out
}

func cloneBoolMatrixWorkflow(values [][]bool) [][]bool {
	out := make([][]bool, len(values))
	for i := range values {
		out[i] = append([]bool(nil), values[i]...)
	}
	return out
}

func (w *BlastWizard) snapshotKeywordSequenceCache(ctx context.Context, selected model.SpeciesCandidate, rows []model.KeywordResultRow) (*sessionsnapshot.SequenceCacheV1, error) {
	results := w.prefetchKeywordSequences(ctx, selected, rows, nil)
	targetID := keywordSequenceFetchTargetID(w.source, selected)
	seenIDs := make(map[string]struct{}, len(rows))
	entries := make([]sessionsnapshot.SequenceCacheEntryV1, 0, len(results))
	for _, row := range rows {
		sequenceID := strings.TrimSpace(row.SequenceID)
		if sequenceID == "" {
			continue
		}
		if _, ok := seenIDs[sequenceID]; ok {
			continue
		}
		seenIDs[sequenceID] = struct{}{}
		fetched, ok := results[sequenceID]
		if !ok {
			continue
		}
		if fetched.err != nil {
			continue
		}
		sequence := strings.TrimSpace(fetched.data.Sequence)
		if sequence == "" {
			continue
		}
		entries = append(entries, sessionsnapshot.SequenceCacheEntryV1{
			TargetID:       targetID,
			SequenceID:     sequenceID,
			Sequence:       sequence,
			OriginalHeader: firstNonEmpty(strings.TrimSpace(fetched.data.OriginalHeader), keywordProteinSequenceHeader(row)),
		})
	}
	for _, row := range rows {
		if inline := inlineKeywordSnapshotSequenceEntry(targetID, row); inline != nil {
			if _, ok := seenIDs[inline.SequenceID]; ok {
				continue
			}
			seenIDs[inline.SequenceID] = struct{}{}
			entries = append(entries, *inline)
		}
	}
	return &sessionsnapshot.SequenceCacheV1{Entries: entries}, nil
}

func (w *BlastWizard) snapshotBlastSequenceCache(ctx context.Context, runs []blastQueryRun) (*sessionsnapshot.SequenceCacheV1, error) {
	rows := make([]model.BlastResultRow, 0)
	for _, run := range runs {
		rows = append(rows, run.Results.Rows...)
	}
	results := w.prefetchBlastSequences(ctx, rows, nil)
	seen := make(map[string]struct{}, len(rows))
	entries := make([]sessionsnapshot.SequenceCacheEntryV1, 0, len(results))
	for _, row := range rows {
		sequenceID := strings.TrimSpace(firstNonEmpty(row.SequenceID, row.TranscriptID, row.Protein))
		if sequenceID == "" {
			continue
		}
		key := fmt.Sprintf("%d:%s", row.TargetID, sequenceID)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		fetched, ok := results[key]
		if !ok {
			continue
		}
		if fetched.err != nil {
			if !isMissingProteinSequenceError(fetched.err) {
				return nil, fmt.Errorf("BLAST snapshot sequence %s: %w", sequenceID, fetched.err)
			}
			continue
		}
		sequence := strings.TrimSpace(fetched.data.Sequence)
		if sequence == "" {
			continue
		}
		entries = append(entries, sessionsnapshot.SequenceCacheEntryV1{
			TargetID:       row.TargetID,
			SequenceID:     sequenceID,
			Sequence:       sequence,
			OriginalHeader: firstNonEmpty(strings.TrimSpace(fetched.data.OriginalHeader), blastProteinSequenceHeader(row)),
		})
	}
	return &sessionsnapshot.SequenceCacheV1{Entries: entries}, nil
}

func (w *BlastWizard) snapshotKeywordSequenceCacheWithModal(ctx context.Context, selected model.SpeciesCandidate, rows []model.KeywordResultRow) (*sessionsnapshot.SequenceCacheV1, error) {
	if w.suppressTaskModals {
		return w.snapshotKeywordSequenceCache(ctx, selected, rows)
	}
	return tui.RunProgressTaskValueContext(tui.TaskPage{
		Path:        w.tuiPath("Export", "Session snapshot"),
		Title:       "Collecting keyword sequences",
		Description: "Gathering the peptide sequences needed to freeze the current keyword review state.",
		Initial:     "Collecting keyword sequences...",
		Total:       maxInt(1, len(rows)),
		CancelError: prompt.ErrBackToRowSelection,
	}, func(taskCtx context.Context, update func(int, string)) (*sessionsnapshot.SequenceCacheV1, error) {
		progress := safeProgress(update)
		progress(0, "Collecting keyword sequences...")
		cache, err := w.snapshotKeywordSequenceCache(mergeContexts(ctx, taskCtx), selected, rows)
		if err != nil {
			return nil, err
		}
		progress(maxInt(1, len(rows)), "Keyword sequences collected.")
		return cache, nil
	})
}

func (w *BlastWizard) snapshotBlastSequenceCacheWithModal(ctx context.Context, runs []blastQueryRun) (*sessionsnapshot.SequenceCacheV1, error) {
	if w.suppressTaskModals {
		return w.snapshotBlastSequenceCache(ctx, runs)
	}
	rowCount := 0
	for _, run := range runs {
		rowCount += len(run.Results.Rows)
	}
	return tui.RunProgressTaskValueContext(tui.TaskPage{
		Path:        w.tuiPath("Export", "Session snapshot"),
		Title:       "Collecting BLAST sequences",
		Description: "Gathering the peptide sequences needed to freeze the current BLAST review state.",
		Initial:     "Collecting BLAST sequences...",
		Total:       maxInt(1, rowCount),
		CancelError: prompt.ErrBackToRowSelection,
	}, func(taskCtx context.Context, update func(int, string)) (*sessionsnapshot.SequenceCacheV1, error) {
		progress := safeProgress(update)
		progress(0, "Collecting BLAST sequences...")
		cache, err := w.snapshotBlastSequenceCache(mergeContexts(ctx, taskCtx), runs)
		if err != nil {
			return nil, err
		}
		progress(maxInt(1, rowCount), "BLAST sequences collected.")
		return cache, nil
	})
}

func (w *BlastWizard) hydrateSnapshotSequenceCache(cache *sessionsnapshot.SequenceCacheV1) {
	if cache == nil {
		return
	}
	for _, entry := range cache.Entries {
		sequenceID := strings.TrimSpace(entry.SequenceID)
		sequence := strings.TrimSpace(entry.Sequence)
		if sequenceID == "" || sequence == "" {
			continue
		}
		w.storeProteinSequence(w.proteinSequenceCacheKey(entry.TargetID, sequenceID), model.ProteinSequenceData{
			Sequence:       sequence,
			OriginalHeader: strings.TrimSpace(entry.OriginalHeader),
		})
	}
}

func inlineKeywordSnapshotSequenceEntry(targetID int, row model.KeywordResultRow) *sessionsnapshot.SequenceCacheEntryV1 {
	sequenceID := strings.TrimSpace(row.SequenceID)
	sequence := strings.TrimSpace(extractInlineKeywordSequence(row))
	if sequenceID == "" || sequence == "" {
		return nil
	}
	return &sessionsnapshot.SequenceCacheEntryV1{
		TargetID:       targetID,
		SequenceID:     sequenceID,
		Sequence:       sequence,
		OriginalHeader: keywordProteinSequenceHeader(row),
	}
}

func extractInlineKeywordSequence(row model.KeywordResultRow) string {
	extraKeys := []string{
		"protein_sequence",
		"sequence",
		"peptide_sequence",
		"fasta_sequence",
		"attr_translation",
	}
	for _, key := range extraKeys {
		if row.ExtraColumns != nil {
			if value := strings.TrimSpace(row.ExtraColumns[key]); value != "" {
				return value
			}
		}
	}
	return ""
}

func snapshotSequenceEntryDedupKey(entry sessionsnapshot.SequenceCacheEntryV1) string {
	return strconv.Itoa(entry.TargetID) + ":" + strings.TrimSpace(entry.SequenceID)
}

func normalizeSnapshotDatabase(database string) string {
	switch strings.ToLower(strings.TrimSpace(database)) {
	case "phytozome":
		return "phytozome"
	case "lemna", "lemna.org":
		return "lemna"
	case "tair", "arabidopsis":
		return "tair"
	default:
		return strings.ToLower(strings.TrimSpace(database))
	}
}

func inferSnapshotDatabase(snapshot sessionsnapshot.Snapshot) string {
	if snapshot.Keyword != nil {
		for _, row := range flattenKeywordSearchGroups(snapshot.Keyword.Groups) {
			if database := normalizeSnapshotDatabase(row.SourceDatabase); database != "" {
				return database
			}
		}
	}
	if snapshot.Blast != nil {
		for _, run := range snapshot.Blast.Runs {
			for _, row := range run.Results.Rows {
				if database := normalizeSnapshotDatabase(row.SourceDatabase); database != "" {
					return database
				}
			}
		}
	}
	return ""
}
