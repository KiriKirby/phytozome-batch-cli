program mega_phgo_runtime;

{$mode objfpc}{$H+}

uses
  {$IFDEF UNIX}
  cthreads,
  {$ENDIF}
  Classes, SysUtils, Process, fpjson, jsonparser, Interfaces, MAlnThread, MD_Sequences,
  MegaConsts,
  MegaUtils, MDistPack, MSeqDistBase, MNucDist, MAminoDist, MTreeEstFunc, MTreeSearchThread, MTreeList, MTreeData,
  MPTree, MLTree, MLModels, MLTreeAnalyzer, MLSearchThread, MRuntimeProgressDlg, MAnalysisInfo, MPartitionList, MTreePack,
  MD_InputSeqData, MOtuInfo, MDomainInfo, MProcessPack, ParsimSearchThreads;

const
  RuntimeName = 'mega-phgo-runtime';
  RuntimeProbeArgument = '--phgo-runtime-probe';

function RuntimeTimestamp(const Value: TDateTime): String;
begin
  Result := FormatDateTime('yyyy"-"mm"-"dd"T"hh":"nn":"ss"."zzz', Value) + 'Z';
end;

procedure WaitForRuntimeThread(Thread: TThread);
begin
  if not Assigned(Thread) then
    Exit;
  while not Thread.Finished do
  begin
    CheckSynchronize(10);
    Sleep(10);
  end;
  CheckSynchronize;
  Thread.WaitFor;
end;

type
  TRuntimePaths = record
    BaseDir: String;
    InputFasta: String;
    MetadataJson: String;
    AlignedFasta: String;
    Newick: String;
    Summary: String;
    RuntimeLog: String;
  end;

  TInputRecord = record
    TaxonID: String;
    DisplayName: String;
    SequenceKind: String;
    CanvasItem: String;
    CanvasRow: Integer;
  end;

  TInputRecords = array of TInputRecord;

  TSkippedRuntimeRecord = record
    TaxonID: String;
    ItemTitle: String;
    RowIndex: Integer;
    Reason: String;
  end;

  TSkippedRuntimeRecords = array of TSkippedRuntimeRecord;

  TRuntimeRequest = record
    SchemaVersion: Integer;
    SessionID: String;
    RunID: String;
    SequenceKind: String;
    ConversionTarget: String;
    AlignmentMethod: String;
    AlignmentParamsJSON: String;
    TreeMethod: String;
    TreeParamsJSON: String;
    InputFasta: String;
    Records: TInputRecords;
    Artifacts: TRuntimePaths;
  end;

  TPHgoMLTreeSearchThread = class(TMLTreeSearchThread)
  public
    function RunPHgoSearch(const Request: TRuntimeRequest; MakeInitialTree: Boolean): Boolean;
  end;

  TPHgoBootstrapMLThread = class(TBootstrapMLThread)
  protected
    function Initialize: Boolean; override;
  public
    function RunPHgoBootstrap(const Request: TRuntimeRequest): Boolean;
  end;

procedure AppendRuntimeLog(const Request: TRuntimeRequest; const MessageText: String); forward;

function RuntimeRecordReason(const BaseReason: String; const RecordIndex: Integer): String;
begin
  Result := Trim(BaseReason);
  if Result = '' then
    Result := 'MEGA reported this row as incompatible with the current tree computation';
  if RecordIndex >= 0 then
    Result := Result + ' (MEGA sequence ' + IntToStr(RecordIndex + 1) + ')';
end;

function RuntimeSkippedRecordForIndex(const Request: TRuntimeRequest; const RecordIndex: Integer;
  const Reason: String): TSkippedRuntimeRecord;
begin
  Result.TaxonID := '';
  Result.ItemTitle := '';
  Result.RowIndex := RecordIndex;
  Result.Reason := RuntimeRecordReason(Reason, RecordIndex);
  if (RecordIndex >= 0) and (RecordIndex < Length(Request.Records)) then
  begin
    Result.TaxonID := Request.Records[RecordIndex].TaxonID;
    Result.ItemTitle := Request.Records[RecordIndex].CanvasItem;
    Result.RowIndex := Request.Records[RecordIndex].CanvasRow;
    Result.Reason := RuntimeRecordReason(Reason, RecordIndex);
  end
end;

procedure AddRuntimeSkippedRecord(var Skipped: TSkippedRuntimeRecords; const RecordValue: TSkippedRuntimeRecord);
var
  I, N: Integer;
begin
  for I := 0 to Length(Skipped) - 1 do
    if (Skipped[I].TaxonID = RecordValue.TaxonID) and
       (Skipped[I].ItemTitle = RecordValue.ItemTitle) and
       (Skipped[I].RowIndex = RecordValue.RowIndex) then
      Exit;
  N := Length(Skipped);
  SetLength(Skipped, N + 1);
  Skipped[N] := RecordValue;
end;

function ParseRuntimeSequencePairError(const ErrorText: String; out FirstIndex, SecondIndex: Integer): Boolean;
var
  OpenPos, ClosePos, CommaPos, I: Integer;
  Inside, LeftText, RightText: String;
begin
  Result := False;
  FirstIndex := -1;
  SecondIndex := -1;
  if Pos('sequence pair', LowerCase(ErrorText)) < 1 then
    Exit;
  OpenPos := 0;
  ClosePos := 0;
  for I := Length(ErrorText) downto 1 do
  begin
    if (ClosePos = 0) and (ErrorText[I] = ')') then
      ClosePos := I
    else if (ClosePos > 0) and (ErrorText[I] = '(') then
    begin
      OpenPos := I;
      Break;
    end;
  end;
  if (OpenPos < 1) or (ClosePos <= OpenPos) then
    Exit;
  Inside := Copy(ErrorText, OpenPos + 1, ClosePos - OpenPos - 1);
  CommaPos := Pos(',', Inside);
  if CommaPos < 1 then
    Exit;
  LeftText := Trim(Copy(Inside, 1, CommaPos - 1));
  RightText := Trim(Copy(Inside, CommaPos + 1, MaxInt));
  Result := TryStrToInt(LeftText, FirstIndex) and TryStrToInt(RightText, SecondIndex);
  if Result then
  begin
    Dec(FirstIndex);
    Dec(SecondIndex);
  end;
end;

function ParseRuntimeSingleSequenceError(const ErrorText: String; out RecordIndex: Integer): Boolean;
var
  LowerText, NumberText: String;
  PosSequence, I: Integer;
begin
  Result := False;
  RecordIndex := -1;
  LowerText := LowerCase(ErrorText);
  PosSequence := Pos('sequence ', LowerText);
  if PosSequence < 1 then
    Exit;
  I := PosSequence + Length('sequence ');
  NumberText := '';
  while (I <= Length(ErrorText)) and (ErrorText[I] in ['0'..'9']) do
  begin
    NumberText := NumberText + ErrorText[I];
    Inc(I);
  end;
  Result := TryStrToInt(NumberText, RecordIndex);
  if Result then
    Dec(RecordIndex);
end;

function RuntimeSkippedRecordsForError(const Request: TRuntimeRequest; const ErrorText: String): TSkippedRuntimeRecords;
var
  FirstIndex, SecondIndex, RecordIndex: Integer;
  Records: TSkippedRuntimeRecords;
begin
  Records := nil;
  SetLength(Records, 0);
  if ParseRuntimeSequencePairError(ErrorText, FirstIndex, SecondIndex) then
  begin
    AddRuntimeSkippedRecord(Records, RuntimeSkippedRecordForIndex(Request, FirstIndex, ErrorText));
    AddRuntimeSkippedRecord(Records, RuntimeSkippedRecordForIndex(Request, SecondIndex, ErrorText));
  end;
  if ParseRuntimeSingleSequenceError(ErrorText, RecordIndex) then
    AddRuntimeSkippedRecord(Records, RuntimeSkippedRecordForIndex(Request, RecordIndex, ErrorText));
  Result := Records;
end;

function TPHgoMLTreeSearchThread.RunPHgoSearch(const Request: TRuntimeRequest; MakeInitialTree: Boolean): Boolean;
begin
  Result := False;
  AppendRuntimeLog(Request, 'maximum_likelihood.thread initialize.start');
  if not Initialize then
    raise Exception.Create('Initialization of MEGA maximum-likelihood tree search failed');
  AppendRuntimeLog(Request, 'maximum_likelihood.thread initialize.complete');
  MLTreeAnalyzer.CheckCancel := @CheckCancel;
  AppendRuntimeLog(Request, 'maximum_likelihood.prepare.start');
  MLTreeAnalyzer.PrepareSearchMLTree(MakeInitialTree);
  AppendRuntimeLog(Request, 'maximum_likelihood.prepare.complete');
  AppendRuntimeLog(Request, 'maximum_likelihood.search.start');
  Search;
  AppendRuntimeLog(Request, 'maximum_likelihood.search.returned');
  if Canceled then
    raise Exception.Create('MEGA maximum-likelihood tree search was cancelled');
  AppendRuntimeLog(Request, 'maximum_likelihood.search.canceled_checked');
  AppendRuntimeLog(Request, 'maximum_likelihood.search.success=true');
  Result := True;
end;

function TPHgoBootstrapMLThread.Initialize: Boolean;
begin
  Result := False;
  if MLTreeAnalyzer = nil then
    raise Exception.Create('MEGA maximum-likelihood bootstrap has no analyzer');
  if MLTreeAnalyzer.Model = nil then
    raise Exception.Create('MEGA maximum-likelihood bootstrap has no substitution model');
  if ProgressDlg <> nil then
    FAnalysisInfo := TAnalysisInfo(ProgressDlg.FMAI);
  GetMem(FBootTable, SizeOf(Integer) * (MLTreeAnalyzer.Model.NoOfSites + 1));
  MLTreeAnalyzer.CheckCancel := @MyCheckCancel;
  MLTreeAnalyzer.SubTaskCheckCancel := @SubTaskCheckCancelFunc;
  if Assigned(FAnalysisInfo) then
    FAnalysisInfo.MyMLAnalysisPack := MLTreeAnalyzer;
  Result := True;
end;

function TPHgoBootstrapMLThread.RunPHgoBootstrap(const Request: TRuntimeRequest): Boolean;
begin
  Result := False;
  AppendRuntimeLog(Request, 'maximum_likelihood.bootstrap.thread start');
  Search;
  if Canceled then
    raise Exception.Create('MEGA maximum-likelihood bootstrap was cancelled');
  AppendRuntimeLog(Request, 'maximum_likelihood.bootstrap.thread complete');
  Result := True;
end;

function JsonString(Obj: TJSONObject; const Name: String; const Default: String = ''): String;
var
  Data: TJSONData;
begin
  Result := Default;
  if Obj = nil then
    Exit;
  Data := Obj.Find(Name);
  if Assigned(Data) then
    Result := Data.AsString;
end;

function JsonInt(Obj: TJSONObject; const Name: String; const Default: Integer = 0): Integer;
var
  Data: TJSONData;
begin
  Result := Default;
  if Obj = nil then
    Exit;
  Data := Obj.Find(Name);
  if Assigned(Data) then
    Result := Data.AsInteger;
end;

function JsonObject(Obj: TJSONObject; const Name: String): TJSONObject;
var
  Data: TJSONData;
begin
  Result := nil;
  if Obj = nil then
    Exit;
  Data := Obj.Find(Name);
  if Assigned(Data) and (Data.JSONType = jtObject) then
    Result := TJSONObject(Data);
end;

function JsonArray(Obj: TJSONObject; const Name: String): TJSONArray;
var
  Data: TJSONData;
begin
  Result := nil;
  if Obj = nil then
    Exit;
  Data := Obj.Find(Name);
  if Assigned(Data) and (Data.JSONType = jtArray) then
    Result := TJSONArray(Data);
end;

function JsonObjectText(Obj: TJSONObject; const Name: String): String;
var
  Data: TJSONData;
begin
  Result := '';
  if Obj = nil then
    Exit;
  Data := Obj.Find(Name);
  if Assigned(Data) and (Data.JSONType = jtObject) then
    Result := Data.AsJSON;
end;

function ParamsObject(const Request: TRuntimeRequest): TJSONObject;
var
  Data: TJSONData;
begin
  Result := nil;
  if Trim(Request.AlignmentParamsJSON) = '' then
    Exit;
  Data := GetJSON(Request.AlignmentParamsJSON);
  if Data is TJSONObject then
    Result := TJSONObject(Data)
  else
    Data.Free;
end;

function TreeParamsObject(const Request: TRuntimeRequest): TJSONObject;
var
  Data: TJSONData;
begin
  Result := nil;
  if Trim(Request.TreeParamsJSON) = '' then
    Exit;
  Data := GetJSON(Request.TreeParamsJSON);
  if Data is TJSONObject then
    Result := TJSONObject(Data)
  else
    Data.Free;
end;

function RuntimeExecutableDir: String;
begin
  Result := ExtractFileDir(ExpandFileName(ParamStr(0)));
end;

function RuntimeOwnedMuscleExecutable: String;
begin
  {$IFDEF MSWINDOWS}
  Result := IncludeTrailingPathDelimiter(RuntimeExecutableDir) + 'muscleWin64.bin';
  {$ELSE}
    {$IFDEF DARWIN}
    Result := IncludeTrailingPathDelimiter(RuntimeExecutableDir) + 'muscledarwin64';
    {$ELSE}
    Result := IncludeTrailingPathDelimiter(RuntimeExecutableDir) + 'muscleUnix64.exe';
    {$ENDIF}
  {$ENDIF}
end;

procedure AppendRuntimeLog(const Request: TRuntimeRequest; const MessageText: String);
var
  List: TStringList;
begin
  if Trim(Request.Artifacts.RuntimeLog) = '' then
    Exit;
  if ExtractFileDir(Request.Artifacts.RuntimeLog) <> '' then
    ForceDirectories(ExtractFileDir(Request.Artifacts.RuntimeLog));
  List := TStringList.Create;
  try
    if FileExists(Request.Artifacts.RuntimeLog) then
      List.LoadFromFile(Request.Artifacts.RuntimeLog);
    List.Add(RuntimeTimestamp(Now) + ' ' + MessageText);
    List.SaveToFile(Request.Artifacts.RuntimeLog);
  finally
    List.Free;
  end;
end;

function ParamString(Params: TJSONObject; const Name: String; const Default: String): String;
begin
  Result := JsonString(Params, Name, Default);
end;

function ParamFloat(Params: TJSONObject; const Name: String; const Default: Double): Double;
var
  S: String;
  Settings: TFormatSettings;
begin
  Result := Default;
  Settings := DefaultFormatSettings;
  Settings.DecimalSeparator := '.';
  S := Trim(ParamString(Params, Name, ''));
  if S = '' then
    Exit;
  if not TryStrToFloat(S, Result, Settings) then
    raise Exception.Create('invalid float alignment parameter ' + Name + ': ' + S);
end;

function ParamInt(Params: TJSONObject; const Name: String; const Default: Integer): Integer;
var
  S: String;
begin
  Result := Default;
  S := Trim(ParamString(Params, Name, ''));
  if S = '' then
    Exit;
  if not TryStrToInt(S, Result) then
    raise Exception.Create('invalid integer alignment parameter ' + Name + ': ' + S);
end;

function ParamBool(Params: TJSONObject; const Name: String; const Default: Boolean): Boolean;
var
  S: String;
begin
  Result := Default;
  S := LowerCase(Trim(ParamString(Params, Name, '')));
  if S = '' then
    Exit;
  if (S = 'true') or (S = 'on') or (S = 'yes') or (S = '1') then
    Exit(True);
  if (S = 'false') or (S = 'off') or (S = 'no') or (S = '0') then
    Exit(False);
  raise Exception.Create('invalid boolean alignment parameter ' + Name + ': ' + S);
end;

function LoadTextFile(const FileName: String): String;
var
  List: TStringList;
begin
  List := TStringList.Create;
  try
    List.LoadFromFile(FileName);
    Result := List.Text;
  finally
    List.Free;
  end;
end;

procedure SaveTextFile(const FileName: String; const Text: String);
var
  List: TStringList;
begin
  if ExtractFileDir(FileName) <> '' then
    ForceDirectories(ExtractFileDir(FileName));
  List := TStringList.Create;
  try
    List.Text := Text;
    List.SaveToFile(FileName);
  finally
    List.Free;
  end;
end;

function MegaAlignedSiteCount(Seqs: TStringList): Integer; forward;
procedure FreeMegaMappedData(var MappedData: TList); forward;

function IsAbsolutePath(const PathValue: String): Boolean;
begin
  Result := False;
  if PathValue = '' then
    Exit;
  {$IFDEF MSWINDOWS}
  Result := ((Length(PathValue) >= 2) and (PathValue[2] = ':')) or
            ((Length(PathValue) >= 2) and (PathValue[1] = '\') and (PathValue[2] = '\')) or
            ((Length(PathValue) >= 2) and (PathValue[1] = '/') and (PathValue[2] = '/'));
  {$ELSE}
  Result := PathValue[1] = '/';
  {$ENDIF}
end;

function NormalizePath(const BaseDir: String; const PathValue: String): String;
begin
  Result := Trim(PathValue);
  if Result = '' then
    Exit;
  if not IsAbsolutePath(Result) then
    Result := ExpandFileName(IncludeTrailingPathDelimiter(BaseDir) + Result);
end;

function ParseRequest(const RequestPath: String): TRuntimeRequest;
var
  Root, Settings, Artifacts, RecordObj: TJSONObject;
  Records: TJSONArray;
  Data: TJSONData;
  I: Integer;
begin
  Result.SchemaVersion := 0;
  Result.SessionID := '';
  Result.RunID := '';
  Result.SequenceKind := '';
  Result.InputFasta := '';
  Result.ConversionTarget := '';
  Result.AlignmentMethod := '';
  Result.AlignmentParamsJSON := '';
  Result.TreeMethod := '';
  Result.TreeParamsJSON := '';
  Result.Artifacts.BaseDir := '';
  Result.Artifacts.AlignedFasta := '';
  Result.Artifacts.Newick := '';
  Result.Artifacts.Summary := '';
  Result.Artifacts.RuntimeLog := '';
  SetLength(Result.Records, 0);
  Data := GetJSON(LoadTextFile(RequestPath));
  try
    if not (Data is TJSONObject) then
      raise Exception.Create('runtime request root must be a JSON object');
    Root := TJSONObject(Data);
    Result.SchemaVersion := JsonInt(Root, 'schema_version', 0);
    Result.SessionID := JsonString(Root, 'session_id');
    Result.RunID := JsonString(Root, 'run_id');
    Result.SequenceKind := JsonString(Root, 'sequence_kind');
    Result.InputFasta := JsonString(Root, 'input_fasta');

    Settings := JsonObject(Root, 'settings');
    Result.ConversionTarget := JsonString(Settings, 'conversion_target', Result.SequenceKind);
    Result.AlignmentMethod := JsonString(Settings, 'alignment_method', 'clustalw');
    Result.AlignmentParamsJSON := JsonObjectText(Settings, 'alignment_params');
    Result.TreeMethod := JsonString(Settings, 'tree_method', 'neighbor_joining');
    Result.TreeParamsJSON := JsonObjectText(Settings, 'tree_params');

    Artifacts := JsonObject(Root, 'artifacts');
    Result.Artifacts.BaseDir := JsonString(Artifacts, 'base_dir', ExtractFileDir(RequestPath));
    Result.Artifacts.InputFasta := NormalizePath(Result.Artifacts.BaseDir, JsonString(Artifacts, 'input_fasta'));
    Result.Artifacts.MetadataJson := NormalizePath(Result.Artifacts.BaseDir, JsonString(Artifacts, 'metadata_json'));
    Result.Artifacts.AlignedFasta := NormalizePath(Result.Artifacts.BaseDir, JsonString(Artifacts, 'aligned_fasta'));
    Result.Artifacts.Newick := NormalizePath(Result.Artifacts.BaseDir, JsonString(Artifacts, 'newick'));
    Result.Artifacts.Summary := NormalizePath(Result.Artifacts.BaseDir, JsonString(Artifacts, 'summary'));
    Result.Artifacts.RuntimeLog := NormalizePath(Result.Artifacts.BaseDir, JsonString(Artifacts, 'runtime_log'));

    Records := JsonArray(Root, 'records');
    if Records <> nil then
    begin
      SetLength(Result.Records, Records.Count);
      for I := 0 to Records.Count - 1 do
      begin
        if not (Records.Items[I] is TJSONObject) then
          Continue;
        RecordObj := TJSONObject(Records.Items[I]);
        Result.Records[I].TaxonID := JsonString(RecordObj, 'taxon_id');
        Result.Records[I].DisplayName := JsonString(RecordObj, 'display_name');
        Result.Records[I].SequenceKind := JsonString(RecordObj, 'sequence_kind');
        Result.Records[I].CanvasItem := JsonString(RecordObj, 'canvas_item');
        Result.Records[I].CanvasRow := JsonInt(RecordObj, 'canvas_row', I);
      end;
    end;
  finally
    Data.Free;
  end;
end;

procedure ParseFasta(const Fasta: String; Names, Seqs: TStringList);
var
  Lines: TStringList;
  I: Integer;
  CurrentName: String;
  CurrentSeq: String;
  Line: String;

  procedure Flush;
  begin
    if CurrentName <> '' then
    begin
      Names.Add(CurrentName);
      Seqs.Add(CurrentSeq);
    end;
    CurrentName := '';
    CurrentSeq := '';
  end;

begin
  Lines := TStringList.Create;
  try
    Lines.Text := Fasta;
    for I := 0 to Lines.Count - 1 do
    begin
      Line := Trim(Lines[I]);
      if Line = '' then
        Continue;
      if Line[1] = '>' then
      begin
        Flush;
        CurrentName := Trim(Copy(Line, 2, MaxInt));
      end
      else
        CurrentSeq := CurrentSeq + UpperCase(Line);
    end;
    Flush;
  finally
    Lines.Free;
  end;
end;

function WrapSequence(const Seq: String): String;
var
  I: Integer;
begin
  Result := '';
  I := 1;
  while I <= Length(Seq) do
  begin
    Result := Result + Copy(Seq, I, 80) + LineEnding;
    Inc(I, 80);
  end;
end;

function FastaFromLists(Names, Seqs: TStringList): String;
var
  I: Integer;
begin
  Result := '';
  for I := 0 to Names.Count - 1 do
    Result := Result + '>' + Names[I] + LineEnding + WrapSequence(Seqs[I]);
end;

function RuntimeTreeGapsMissingData(const Request: TRuntimeRequest): String;
var
  Params: TJSONObject;
  DefaultValue, Method: String;
begin
  Method := LowerCase(Trim(Request.TreeMethod));
  if (Method = 'maximum_likelihood') or (Method = 'maximum_parsimony') then
    DefaultValue := 'Use all sites'
  else
    DefaultValue := 'Pairwise deletion';
  Params := TreeParamsObject(Request);
  try
    Result := ParamString(Params, 'gaps_missing_data', DefaultValue);
  finally
    Params.Free;
  end;
  if Trim(Result) = '' then
    Result := DefaultValue;
end;

function RuntimeTreeSiteCoverage(const Request: TRuntimeRequest): Integer;
var
  Params: TJSONObject;
begin
  Params := TreeParamsObject(Request);
  try
    Result := ParamInt(Params, 'site_coverage_cutoff', 95);
  finally
    Params.Free;
  end;
  if Result < 0 then
    Result := 0;
  if Result > 100 then
    Result := 100;
end;

function RuntimeTreeSubsetOptions(const Request: TRuntimeRequest; IsNucleotide: Boolean; const PrepKind: String): TDataSubsetOptions;
var
  GapsMissing: String;
begin
  Result := [];
  if IsNucleotide then
    Result := Result + [dsoUseNuc, dsoUseNonCod]
  else
    Result := Result + [dsoUseAmino];

  if SameText(PrepKind, 'distance') then
    Result := Result + [dsoDistMap]
  else if SameText(PrepKind, 'parsimony') then
    Result := Result + [dsoParsimMap]
  else
    Result := Result + [dsoNoMap];

  GapsMissing := LowerCase(Trim(RuntimeTreeGapsMissingData(Request)));
  if Pos('complete', GapsMissing) > 0 then
    Result := Result + [dsoCompleteDeletion]
  else if Pos('partial', GapsMissing) > 0 then
    Result := Result + [dsoPartialDeletion]
  else if Pos('pairwise', GapsMissing) > 0 then
    Result := Result + [dsoPairwiseDeletion];
end;

function CreateRuntimeInputSeqData(Names, Seqs: TStringList; IsNucleotide: Boolean): TD_InputSeqData;
var
  OtuInfos: TAllOtuInfo;
  Info: TOtuInfo;
  SeqBuffer: PAnsiChar;
  Seq: String;
  I, J, NoOfSites: Integer;
begin
  if Names.Count <> Seqs.Count then
    raise Exception.Create('MEGA runtime input names and sequences are out of sync');
  NoOfSites := MegaAlignedSiteCount(Seqs);
  Result := TD_InputSeqData.CreateWithoutGui;
  try
    Result.IsNuc := IsNucleotide;
    Result.IsAmino := not IsNucleotide;
    Result.IsCoding := False;
    Result.GapSym := '-';
    Result.MissSym := '?';
    Result.IdenSym := '.';
    Result.NoOfTaxa := Names.Count;
    Result.NoOfSites := NoOfSites;
    Result.NoOfNucSites := NoOfSites;
    Result.NoOfSitesUsed := NoOfSites;
    Result.DomainMarks := TAllSiteDomainMark.Create;
    Result.DomainMarks.NoOfSites := NoOfSites;

    OtuInfos := TAllOtuInfo.Create;
    OtuInfos.NoOfOtus := Names.Count;
    for I := 0 to Names.Count - 1 do
    begin
      Seq := Seqs[I];
      Info := TOtuInfo.Create;
      Info.Id := I;
      Info.Name := AnsiString(Names[I]);
      Info.RsvName := AnsiString(Names[I]);
      Info.IsUsed := True;
      GetMem(SeqBuffer, SizeOf(AnsiChar) * (Length(Seq) + 1));
      for J := 1 to Length(Seq) do
        SeqBuffer[J - 1] := AnsiChar(Seq[J]);
      SeqBuffer[Length(Seq)] := #0;
      Info.Data := SeqBuffer;
      OtuInfos[I] := Info;
    end;
    Result.OtuInfos := OtuInfos;
    Result.ReferenceSeq := PAnsiChar(OtuInfos[0].Data);
    D_InputSeqData := Result;
  except
    Result.Free;
    D_InputSeqData := nil;
    raise;
  end;
end;

function RuntimeUsedOtuInfos(InputData: TD_InputSeqData): TList;
var
  I: Integer;
begin
  Result := TList.Create;
  try
    for I := 0 to InputData.OtuInfos.NoOfOtus - 1 do
      if InputData.OtuInfos[I].IsUsed then
        Result.Add(InputData.OtuInfos[I]);
  except
    Result.Free;
    raise;
  end;
end;

function PrepareMegaDistanceMappedData(const Request: TRuntimeRequest; Names, Seqs: TStringList; IsNucleotide: Boolean;
  out NoOfSeqs: Integer; out NoOfSites: Integer): TList;
var
  InputData: TD_InputSeqData;
  UsedOtuInfos: TList;
  Options: TDataSubsetOptions;
begin
  Result := nil;
  InputData := nil;
  UsedOtuInfos := nil;
  try
    try
      InputData := CreateRuntimeInputSeqData(Names, Seqs, IsNucleotide);
      UsedOtuInfos := RuntimeUsedOtuInfos(InputData);
      Options := RuntimeTreeSubsetOptions(Request, IsNucleotide, 'distance');
      Result := TList.Create;
      InputData.PrepareDataForDistAnalysis(Options, Result, UsedOtuInfos, NoOfSeqs, NoOfSites, nil, '', RuntimeTreeSiteCoverage(Request));
      if (NoOfSeqs < 2) or (NoOfSites < 1) then
        raise Exception.Create('MEGA data preparation left insufficient sites or taxa for distance analysis');
      AppendRuntimeLog(Request,
        'tree.data_preparation kind=distance gaps_missing="' + RuntimeTreeGapsMissingData(Request) +
        '" site_coverage=' + IntToStr(RuntimeTreeSiteCoverage(Request)) +
        ' taxa=' + IntToStr(NoOfSeqs) + ' sites=' + IntToStr(NoOfSites));
    except
      FreeMegaMappedData(Result);
      raise;
    end;
  finally
    UsedOtuInfos.Free;
    InputData.Free;
    D_InputSeqData := nil;
  end;
end;

function PrepareMegaParsimonyMappedData(const Request: TRuntimeRequest; Names, Seqs: TStringList; IsNucleotide: Boolean;
  out NoOfSeqs: Integer; out NoOfSites: Integer): TList;
var
  InputData: TD_InputSeqData;
  UsedOtuInfos: TList;
  Options: TDataSubsetOptions;
  InfoSites, ConstContribute: Integer;
begin
  Result := nil;
  InputData := nil;
  UsedOtuInfos := nil;
  try
    try
      InputData := CreateRuntimeInputSeqData(Names, Seqs, IsNucleotide);
      UsedOtuInfos := RuntimeUsedOtuInfos(InputData);
      Options := RuntimeTreeSubsetOptions(Request, IsNucleotide, 'parsimony');
      Result := TList.Create;
      InputData.PrepareDataForParsimAnalysis(Options, Result, UsedOtuInfos, NoOfSeqs, NoOfSites, InfoSites, ConstContribute, nil, '', RuntimeTreeSiteCoverage(Request));
      if NoOfSites < 1 then
        raise Exception.Create('No common sites found');
      if InfoSites < 1 then
        raise Exception.Create('There are no parsimony informative sites');
      if NoOfSeqs < 4 then
        raise Exception.Create('At least four taxa are needed for parsimony analysis');
      AppendRuntimeLog(Request,
        'tree.data_preparation kind=parsimony gaps_missing="' + RuntimeTreeGapsMissingData(Request) +
        '" site_coverage=' + IntToStr(RuntimeTreeSiteCoverage(Request)) +
        ' taxa=' + IntToStr(NoOfSeqs) + ' sites=' + IntToStr(NoOfSites) +
        ' informative_sites=' + IntToStr(InfoSites));
    except
      FreeMegaMappedData(Result);
      raise;
    end;
  finally
    UsedOtuInfos.Free;
    InputData.Free;
    D_InputSeqData := nil;
  end;
end;

procedure PrepareMegaMLSequenceData(const Request: TRuntimeRequest; Names, Seqs: TStringList; IsNucleotide: Boolean;
  out PreparedNames, PreparedSeqs: TStringList);
var
  InputData: TD_InputSeqData;
  UsedOtuInfos: TList;
  Options: TDataSubsetOptions;
  TempSeqList: TList;
  ACharSeq: PAnsiChar;
  AString: AnsiString;
  NoOfSeqs, NoOfSites: Integer;
  I, J: Integer;
begin
  PreparedNames := nil;
  PreparedSeqs := nil;
  InputData := nil;
  UsedOtuInfos := nil;
  TempSeqList := nil;
  try
    try
      InputData := CreateRuntimeInputSeqData(Names, Seqs, IsNucleotide);
      UsedOtuInfos := RuntimeUsedOtuInfos(InputData);
      Options := RuntimeTreeSubsetOptions(Request, IsNucleotide, 'ml');
      TempSeqList := TList.Create;
      InputData.PrepareDataForDistAnalysis(Options, TempSeqList, UsedOtuInfos, NoOfSeqs, NoOfSites, nil, '', RuntimeTreeSiteCoverage(Request));
      if (NoOfSeqs < 2) or (NoOfSites < 1) then
        raise Exception.Create('MEGA data preparation left insufficient sites or taxa for maximum-likelihood analysis');
      if NoOfSeqs < 4 then
        raise Exception.Create('At least four sequences are needed for likelihood tree construction');

      PreparedNames := TStringList.Create;
      PreparedSeqs := TStringList.Create;
      for I := 0 to NoOfSeqs - 1 do
      begin
        PreparedNames.Add(String(TOtuInfo(UsedOtuInfos[I]).Name));
        ACharSeq := TempSeqList[I];
        SetLength(AString, NoOfSites);
        for J := 1 to NoOfSites do
          AString[J] := ACharSeq[J - 1];
        PreparedSeqs.Add(String(AString));
      end;
      AppendRuntimeLog(Request,
        'tree.data_preparation kind=maximum_likelihood gaps_missing="' + RuntimeTreeGapsMissingData(Request) +
        '" site_coverage=' + IntToStr(RuntimeTreeSiteCoverage(Request)) +
        ' taxa=' + IntToStr(PreparedNames.Count) + ' sites=' + IntToStr(NoOfSites));
    except
      PreparedNames.Free;
      PreparedSeqs.Free;
      PreparedNames := nil;
      PreparedSeqs := nil;
      raise;
    end;
  finally
    if TempSeqList <> nil then
    begin
      for I := 0 to TempSeqList.Count - 1 do
        if TempSeqList[I] <> nil then
          FreeMem(TempSeqList[I]);
      TempSeqList.Free;
    end;
    UsedOtuInfos.Free;
    InputData.Free;
    D_InputSeqData := nil;
  end;
end;

function TargetSequenceKind(const Request: TRuntimeRequest): String;
begin
  Result := LowerCase(Trim(Request.ConversionTarget));
  if (Result = '') or (Result = 'protein') or (Result = 'aa') or (Result = 'amino_acid') then
    Exit('protein');
  if (Result = 'dna') or (Result = 'nucleotide') then
    Exit('nucleotide');
  Result := LowerCase(Trim(Request.SequenceKind));
  if Result = 'dna' then
    Result := 'nucleotide';
  if Result <> 'nucleotide' then
    Result := 'protein';
end;

function GeneticCodeTableByName(const CodeName: String): String;
var
  I, Count: Integer;
  Requested, Candidate: String;
begin
  Requested := LowerCase(StringReplace(Trim(CodeName), '_', ' ', [rfReplaceAll]));
  if Requested = '' then
    Requested := 'standard';
  Count := GetNoOfDefaultCodeTables;
  for I := 0 to Count - 1 do
  begin
    Candidate := LowerCase(StringReplace(Trim(GetDefaultCodeTableName(I)), '_', ' ', [rfReplaceAll]));
    if Candidate = Requested then
    begin
      Result := GetDefaultCodeTable(I);
      Exit;
    end;
  end;
  Result := GetStandardGeneticCode;
end;

procedure PrepareCodonAlignment(const Request: TRuntimeRequest; Names, Seqs: TStringList; out SeqList: TSequenceList);
var
  Params: TJSONObject;
  Seq: TSequence;
  I: Integer;
begin
  SeqList := nil;
  Params := nil;
  try
    Params := ParamsObject(Request);
    SeqList := TSequenceList.Create;
    SeqList.IsDNA := True;
    SeqList.IsProteinCoding := True;
    SeqList.CodeTable := AnsiString(GeneticCodeTableByName(ParamString(Params, 'genetic_code', 'Standard')));
    for I := 0 to Seqs.Count - 1 do
    begin
      Seq := TSequence.Create;
      Seq.SeqName := AnsiString(Names[I]);
      Seq.SeqData := AnsiString(UpperCase(Trim(Seqs[I])));
      Seq.CodeTable := SeqList.CodeTable;
      SeqList.Add(Seq);
    end;
    SeqList.Translate;
    for I := 0 to SeqList.Count - 1 do
      Seqs[I] := String(SeqList[I].SeqData);
  finally
    Params.Free;
  end;
end;

procedure FinishCodonAlignment(SeqList: TSequenceList; Seqs: TStringList);
var
  I: Integer;
begin
  if SeqList = nil then
    Exit;
  for I := 0 to SeqList.Count - 1 do
    SeqList[I].SeqData := AnsiString(Seqs[I]);
  SeqList.UnTranslate;
  for I := 0 to SeqList.Count - 1 do
    Seqs[I] := String(SeqList[I].SeqData);
end;

procedure ApplyMegaCodonGapReset(const Request: TRuntimeRequest; Seqs: TStringList; Params: TJSONObject);
var
  I: Integer;
begin
  if ParamBool(Params, 'keep_predefined_gaps', False) then
    Exit;
  for I := 0 to Seqs.Count - 1 do
  begin
    Seqs[I] := String(StripGaps(AnsiString(Seqs[I])));
    if Trim(Seqs[I]) = '' then
      raise Exception.Create('MEGA ClustalW could not remove gaps before codon alignment because a sequence contains only gaps');
  end;
  AppendRuntimeLog(Request, 'clustalw.codons.reset_gaps applied=true');
end;

function ResolveGuideTreeFile(const Request: TRuntimeRequest; const Value: String): String;
var
  Text: TStringList;
begin
  Result := Trim(Value);
  if Result = '' then
    Exit;
  if FileExists(Result) then
    Exit;
  Result := IncludeTrailingPathDelimiter(Request.Artifacts.BaseDir) + 'guide-tree.nwk';
  Text := TStringList.Create;
  try
    Text.Text := Value;
    Text.SaveToFile(Result);
  finally
    Text.Free;
  end;
end;

procedure RunMegaClustalWAlignment(const Request: TRuntimeRequest; Names, Seqs: TStringList);
var
  Thread: TClustalWThread;
  Params: TJSONObject;
  Method, OutputKind, GuideTreeFile: String;
  I, MinLen, MaxLen: Integer;
  ErrorMessage: String;
  GuideTree: TTreeList;
  GuideData: TTreeData;
  CodonSeqList: TSequenceList;
begin
  Method := LowerCase(Trim(Request.AlignmentMethod));
  OutputKind := TargetSequenceKind(Request);
  if (Method <> 'clustalw') and (Method <> 'clustalw_codons') then
    raise Exception.Create('MEGA ClustalW runtime received unsupported method: ' + Request.AlignmentMethod);
  if Seqs.Count < 2 then
    raise Exception.Create('at least two sequences are required for ClustalW alignment');

  Thread := nil;
  Params := nil;
  CodonSeqList := nil;
  try
    Params := ParamsObject(Request);
    if SameText(Method, 'clustalw_codons') then
    begin
      ApplyMegaCodonGapReset(Request, Seqs, Params);
      PrepareCodonAlignment(Request, Names, Seqs, CodonSeqList);
    end;
    MinLen := 0;
    MaxLen := 0;
    for I := 0 to Seqs.Count - 1 do
    begin
      if (I = 0) or (Length(Seqs[I]) < MinLen) then
        MinLen := Length(Seqs[I]);
      if Length(Seqs[I]) > MaxLen then
        MaxLen := Length(Seqs[I]);
    end;
    AppendRuntimeLog(Request,
      'clustalw.start method=' + Method +
      ' sequence_kind=' + OutputKind +
      ' taxa=' + IntToStr(Seqs.Count) +
      ' min_len=' + IntToStr(MinLen) +
      ' max_len=' + IntToStr(MaxLen));
    Thread := TClustalWThread.Create;
    Thread.ShowProgress := False;
    Thread.IsDNA := SameText(OutputKind, 'nucleotide') and not SameText(Method, 'clustalw_codons');
    Thread.SeqList := Seqs;
    Thread.SeqNames := Names;

    Thread.DNAPWGapOpenPenalty := ParamFloat(Params, 'pairwise_gap_opening_penalty', 15.0);
    Thread.DNAPWGapExtendPenalty := ParamFloat(Params, 'pairwise_gap_extension_penalty', 6.66);
    Thread.DNAGapOpenPenalty := ParamFloat(Params, 'multiple_gap_opening_penalty', 15.0);
    Thread.DNAGapExtendPenalty := ParamFloat(Params, 'multiple_gap_extension_penalty', 6.66);
    if SameText(ParamString(Params, 'dna_weight_matrix', 'IUB'), 'ClustalW (1.6)') then
      Thread.DNAMatrix := clustalw
    else
      Thread.DNAMatrix := iub;
    Thread.TransitionWeight := ParamFloat(Params, 'transition_weight', 0.5);
    Thread.ProteinMatrix := gonnet;

    Thread.ProteinPWGapOpenPenalty := ParamFloat(Params, 'pairwise_gap_opening_penalty', 10.0);
    Thread.ProteinPWGapExtendPenalty := ParamFloat(Params, 'pairwise_gap_extension_penalty', 0.1);
    Thread.ProteinGapOpenPenalty := ParamFloat(Params, 'multiple_gap_opening_penalty', 10.0);
    Thread.ProteinGapExtendPenalty := ParamFloat(Params, 'multiple_gap_extension_penalty', 0.2);

    if SameText(ParamString(Params, 'protein_weight_matrix', 'BLOSUM'), 'PAM') then
      Thread.ProteinMatrix := pam
    else if SameText(ParamString(Params, 'protein_weight_matrix', 'BLOSUM'), 'Gonnet') then
      Thread.ProteinMatrix := gonnet
    else if SameText(ParamString(Params, 'protein_weight_matrix', 'BLOSUM'), 'Identity') then
      Thread.ProteinMatrix := identity
    else
      Thread.ProteinMatrix := blosum;

    Thread.ResidueSpecificPenalty := ParamBool(Params, 'residue_specific_penalties', True);
    Thread.HydrophilicPenalty := ParamBool(Params, 'hydrophilic_penalties', True);
    Thread.GapSeparationDistance := ParamInt(Params, 'gap_separation_distance', 4);
    Thread.EndGapSeparation := ParamBool(Params, 'end_gap_separation', False);
    Thread.UseNegativeMatrix := ParamBool(Params, 'use_negative_matrix', False);
    Thread.DivergentCutoff := ParamInt(Params, 'delay_divergent_cutoff', 30);
    GuideTreeFile := Trim(ParamString(Params, 'guide_tree', ''));
    if GuideTreeFile <> '' then
    begin
      GuideTreeFile := ResolveGuideTreeFile(Request, GuideTreeFile);
      GuideTree := nil;
      GuideData := nil;
      try
        GuideTree := TTreeList.Create;
        if not GuideTree.ImportFromNewickFile(GuideTreeFile, Names, False) then
          raise Exception.Create('MEGA ClustalW could not import the specified guide tree');
        if GuideTree.NoOfTrees < 1 then
          raise Exception.Create('MEGA ClustalW guide tree did not contain any tree');
        GuideData := TTreeData.Create(GuideTree.NoOfOTUs, GuideTree[0].isBLen, GuideTree[0].isSE, GuideTree[0].isStats);
        GuideData.Assign(GuideTree[0]);
        Thread.SetTreeData(GuideData);
      finally
        GuideData.Free;
        GuideTree.Free;
      end;
    end;

    Thread.Start;
    WaitForRuntimeThread(Thread);
    if Thread.Canceled then
    begin
      ErrorMessage := Trim(String(Thread.ErrorMessage));
      if ErrorMessage = '' then
      begin
        ErrorMessage := 'MEGA ClustalW alignment did not complete (taxa=' + IntToStr(Seqs.Count) +
          ', min_len=' + IntToStr(MinLen) + ', max_len=' + IntToStr(MaxLen) + ')';
      end;
      AppendRuntimeLog(Request,
        'clustalw.failed canceled=true error=' + ErrorMessage);
      raise Exception.Create(ErrorMessage);
    end;
    AppendRuntimeLog(Request, 'clustalw.complete canceled=false');
    if SameText(Method, 'clustalw_codons') then
      FinishCodonAlignment(CodonSeqList, Seqs);
  finally
    Thread.Free;
    Params.Free;
    CodonSeqList.Free;
  end;
end;

procedure LoadAlignedFastaIntoLists(const Fasta: String; Names, Seqs: TStringList);
var
  NextNames, NextSeqs: TStringList;
begin
  NextNames := TStringList.Create;
  NextSeqs := TStringList.Create;
  try
    ParseFasta(Fasta, NextNames, NextSeqs);
    if NextNames.Count <> Names.Count then
      raise Exception.Create('MUSCLE returned a different number of sequences than requested');
    Names.Assign(NextNames);
    Seqs.Assign(NextSeqs);
  finally
    NextNames.Free;
    NextSeqs.Free;
  end;
end;

procedure AddMuscleParameter(Process: TProcess; const Name, Value: String);
begin
  if Trim(Value) = '' then
    Exit;
  Process.Parameters.Add(Name);
  Process.Parameters.Add(Trim(Value));
end;

function MuscleClusterValue(const Value: String): String;
begin
  Result := LowerCase(StringReplace(Trim(Value), ' ', '', [rfReplaceAll]));
  if Result = '' then
    Result := 'upgma';
end;

procedure RunRuntimeOwnedMuscleAlignment(const Request: TRuntimeRequest; Names, Seqs: TStringList);
var
  Params: TJSONObject;
  Proc: TProcess;
  InputPath, OutputPath, LogPath, ExePath: String;
  Method: String;
  CodonSeqList: TSequenceList;
begin
  Method := LowerCase(Trim(Request.AlignmentMethod));
  if (Method <> 'muscle') and (Method <> 'muscle_codons') then
    raise Exception.Create('MUSCLE runtime received unsupported method: ' + Request.AlignmentMethod);
  if Seqs.Count < 2 then
    raise Exception.Create('at least two sequences are required for MUSCLE alignment');

  ExePath := RuntimeOwnedMuscleExecutable;
  if not FileExists(ExePath) then
    raise Exception.Create('runtime-owned MUSCLE executable is missing from mega-phgo-runtime: ' + ExePath);

  CodonSeqList := nil;
  if SameText(Method, 'muscle_codons') then
    PrepareCodonAlignment(Request, Names, Seqs, CodonSeqList);

  InputPath := IncludeTrailingPathDelimiter(Request.Artifacts.BaseDir) + 'muscle-input.fasta';
  OutputPath := IncludeTrailingPathDelimiter(Request.Artifacts.BaseDir) + 'muscle-output.fasta';
  LogPath := IncludeTrailingPathDelimiter(Request.Artifacts.BaseDir) + 'muscle.log';
  SaveTextFile(InputPath, FastaFromLists(Names, Seqs));

  Params := nil;
  Proc := nil;
  try
    Params := ParamsObject(Request);
    Proc := TProcess.Create(nil);
    Proc.Executable := ExePath;
    Proc.Options := [poWaitOnExit];
    AddMuscleParameter(Proc, '-in', InputPath);
    AddMuscleParameter(Proc, '-out', OutputPath);
    AddMuscleParameter(Proc, '-log', LogPath);
    AddMuscleParameter(Proc, '-gapopen', FloatToStr(ParamFloat(Params, 'gap_open', -2.9)));
    AddMuscleParameter(Proc, '-gapextend', FloatToStr(ParamFloat(Params, 'gap_extend', 0.0)));
    AddMuscleParameter(Proc, '-hydrofactor', FloatToStr(ParamFloat(Params, 'hydrophobicity_multiplier', 1.2)));
    AddMuscleParameter(Proc, '-maxmb', IntToStr(ParamInt(Params, 'max_memory_mb', 2048)));
    AddMuscleParameter(Proc, '-maxiters', IntToStr(ParamInt(Params, 'max_iterations', 16)));
    AddMuscleParameter(Proc, '-cluster1', MuscleClusterValue(ParamString(Params, 'cluster_method_iterations_1_2', 'UPGMA')));
    AddMuscleParameter(Proc, '-cluster2', MuscleClusterValue(ParamString(Params, 'cluster_method_other_iterations', 'UPGMA')));
    AddMuscleParameter(Proc, '-diaglength', IntToStr(ParamInt(Params, 'min_diag_length_lambda', 24)));
    Proc.Parameters.Add('-quiet');
    Proc.Execute;
    if Proc.ExitStatus <> 0 then
      raise Exception.Create('runtime-owned MUSCLE exited with status ' + IntToStr(Proc.ExitStatus));
    if not FileExists(OutputPath) then
      raise Exception.Create('runtime-owned MUSCLE did not produce an output FASTA file');
    LoadAlignedFastaIntoLists(LoadTextFile(OutputPath), Names, Seqs);
    if SameText(Method, 'muscle_codons') then
      FinishCodonAlignment(CodonSeqList, Seqs);
  finally
    CodonSeqList.Free;
    Proc.Free;
    Params.Free;
  end;
end;

procedure RunAlignment(const Request: TRuntimeRequest; Names, Seqs: TStringList);
var
  Method: String;
begin
  Method := LowerCase(Trim(Request.AlignmentMethod));
  if (Method = '') or (Method = 'clustalw') or (Method = 'clustalw_codons') then
  begin
    RunMegaClustalWAlignment(Request, Names, Seqs);
    Exit;
  end;
  if (Method = 'muscle') or (Method = 'muscle_codons') then
  begin
    RunRuntimeOwnedMuscleAlignment(Request, Names, Seqs);
    Exit;
  end;
  raise Exception.Create('unsupported alignment method: ' + Request.AlignmentMethod);
end;

function RuntimeTreeUsesNucleotide(const Request: TRuntimeRequest): Boolean;
begin
  Result := SameText(TargetSequenceKind(Request), 'nucleotide');
end;

function RuntimeTreeDistanceModel(const Request: TRuntimeRequest; IsNucleotide: Boolean): TDistType;
var
  Params: TJSONObject;
  Model: String;
begin
  Params := TreeParamsObject(Request);
  try
    Model := LowerCase(Trim(ParamString(Params, 'model_method', '')));
    if Model = '' then
      Model := LowerCase(Trim(ParamString(Params, 'distance_model', 'p-distance')));
  finally
    Params.Free;
  end;

  if (Model = '') or (Pos('p-distance', Model) > 0) then
    Exit(gdPropDist);
  if (Pos('no.', Model) > 0) or (Pos('number of differences', Model) > 0) then
    Exit(gdNoOfDiff);
  if IsNucleotide and (Pos('jukes', Model) > 0) then
    Exit(gdJukesCantor);
  if IsNucleotide and (Pos('kimura', Model) > 0) then
    Exit(gdKimura2para);
  if IsNucleotide and (Pos('tajima-nei', Model) > 0) then
    Exit(gdTajimaNei);
  if IsNucleotide and (Pos('tamura 3', Model) > 0) then
    Exit(gdTamura);
  if IsNucleotide and (Pos('tamura-nei', Model) > 0) then
    Exit(gdTamuraNei);
  if IsNucleotide and (Pos('maximum composite', Model) > 0) then
    Exit(gdMCL);
  if IsNucleotide and (Pos('logdet', Model) > 0) then
    Exit(gdLogDet);
  if (not IsNucleotide) and (Pos('poisson', Model) > 0) then
    Exit(gdPoisson);
  if (not IsNucleotide) and (Pos('equal input', Model) > 0) then
    Exit(gdEqualInput);
  if (not IsNucleotide) and (Pos('dayhoff', Model) > 0) then
    Exit(gdDayhoff);
  if (not IsNucleotide) and ((Pos('jones', Model) > 0) or (Pos('jtt', Model) > 0)) then
    Exit(gdJones);
  raise Exception.Create('MEGA distance model is not available for this tree data type: ' + Model);
end;

function BuildMegaDistancePack(const Request: TRuntimeRequest; IsNucleotide: Boolean): TDistPack;
var
  ModelType: TDistType;
  Params: TJSONObject;
  Rates, Pattern: String;
begin
  ModelType := RuntimeTreeDistanceModel(Request, IsNucleotide);
  Result := TDistPack.Create;
  try
    Result.AddType(gdPairwise);
    if IsNucleotide then
      Result.AddType(gdOneNuc)
    else
      Result.AddType(gdAmino);
    Result.AddType(ModelType);
    Params := TreeParamsObject(Request);
    try
      Rates := LowerCase(Trim(ParamString(Params, 'rates_among_sites', 'Uniform Rates')));
      if Pos('gamma distributed', Rates) > 0 then
      begin
        Result.AddType(gdGamma);
        Result.GammaParameter := ParamFloat(Params, 'gamma_parameter', 1.0);
      end;
      Pattern := LowerCase(Trim(ParamString(Params, 'pattern_among_lineages', 'Same (Homogeneous)')));
      if Pos('heterogeneous', Pattern) > 0 then
        Result.AddType(gdHetero);
    finally
      Params.Free;
    end;
  except
    Result.Free;
    raise;
  end;
end;

function MegaAlignedSiteCount(Seqs: TStringList): Integer;
var
  I: Integer;
begin
  Result := 0;
  if Seqs.Count = 0 then
    Exit;
  Result := Length(Seqs[0]);
  for I := 1 to Seqs.Count - 1 do
    if Length(Seqs[I]) <> Result then
      raise Exception.Create('MEGA alignment produced sequences with unequal lengths');
end;

procedure FreeMegaMappedData(var MappedData: TList);
var
  I: Integer;
begin
  if MappedData = nil then
    Exit;
  for I := 0 to MappedData.Count - 1 do
    if MappedData[I] <> nil then
      FreeMem(MappedData[I]);
  MappedData.Free;
  MappedData := nil;
end;

function BuildMegaMappedData(Seqs: TStringList; NoOfSites: Integer; IsNucleotide: Boolean): TList;
var
  I, J: Integer;
  Buffer: PAnsiChar;
  Seq: String;
begin
  Result := TList.Create;
  try
    for I := 0 to Seqs.Count - 1 do
    begin
      Seq := Seqs[I];
      GetMem(Buffer, SizeOf(AnsiChar) * (NoOfSites + 1));
      for J := 1 to NoOfSites do
      begin
        if IsNucleotide then
          Buffer[J - 1] := NucToDistMap(AnsiChar(Seq[J]))
        else
          Buffer[J - 1] := AminoToDistMap(AnsiChar(Seq[J]));
      end;
      Buffer[NoOfSites] := #0;
      Result.Add(Buffer);
    end;
  except
    FreeMegaMappedData(Result);
    raise;
  end;
end;

function BuildMegaParsimonyMappedData(Seqs: TStringList; NoOfSites: Integer; IsNucleotide: Boolean): TList;
var
  I, J: Integer;
  DNA: PAnsiChar;
  Amino: PArrayOfInt;
  Seq: String;
begin
  Result := TList.Create;
  try
    for I := 0 to Seqs.Count - 1 do
    begin
      Seq := Seqs[I];
      if IsNucleotide then
      begin
        GetMem(DNA, SizeOf(AnsiChar) * (NoOfSites + 1));
        for J := 1 to NoOfSites do
          DNA[J - 1] := NucToParsimMap(AnsiChar(Seq[J]));
        DNA[NoOfSites] := #0;
        Result.Add(DNA);
      end
      else
      begin
        GetMem(Amino, SizeOf(Integer) * NoOfSites);
        for J := 1 to NoOfSites do
          Amino^[J - 1] := AminoToParsimMap(AnsiChar(Seq[J]));
        Result.Add(Amino);
      end;
    end;
  except
    FreeMegaMappedData(Result);
    raise;
  end;
end;

function ComputeMegaDistanceMatrix(const Request: TRuntimeRequest; Seqs: TStringList; IsNucleotide: Boolean): PDistanceMatrix;
var
  DistPack: TDistPack;
  DistComputer: TSeqDistBase;
  MappedData: TList;
  NoOfSites, NoOfSeqs: Integer;
begin
  Result := nil;
  DistPack := nil;
  DistComputer := nil;
  MappedData := nil;
  NoOfSeqs := Seqs.Count;
  NoOfSites := MegaAlignedSiteCount(Seqs);
  if NoOfSites < 1 then
    raise Exception.Create('MEGA alignment produced no aligned sites');

  try
    try
      DistPack := BuildMegaDistancePack(Request, IsNucleotide);
      MappedData := BuildMegaMappedData(Seqs, NoOfSites, IsNucleotide);
      Result := NewDistMatrix(NoOfSeqs, True);
      if IsNucleotide then
        DistComputer := TNucDist.Create
      else
        DistComputer := TAminoDist.Create;

      DistComputer.DistPack := DistPack;
      DistComputer.NoOfSeqs := NoOfSeqs;
      DistComputer.Sequences := MappedData;
      DistComputer.QuickExit := True;
      DistComputer.showInProgressBar := False;
      DistComputer.D := Result;
      DistComputer.NoOfSites := NoOfSites;

      if IsNucleotide then
      begin
        if not TNucDist(DistComputer).ComputeDistances then
          raise Exception.Create('MEGA nucleotide distance computation failed');
      end
      else if not TAminoDist(DistComputer).ComputeDistances then
        raise Exception.Create('MEGA amino-acid distance computation failed');
    except
      if Result <> nil then
        DestroyDistanceMatrix(Result, NoOfSeqs);
      raise;
    end;
  finally
    DistComputer.Free;
    DistPack.Free;
    FreeMegaMappedData(MappedData);
  end;
end;

function RuntimeTreeMethod(const Request: TRuntimeRequest): String;
begin
  Result := LowerCase(Trim(Request.TreeMethod));
  if Result = '' then
    Result := 'neighbor_joining';
end;

function RuntimeTreeBootstrapSelected(const Request: TRuntimeRequest): Boolean;
var
  Params: TJSONObject;
  Value: String;
begin
  Result := False;
  Params := TreeParamsObject(Request);
  try
    Value := LowerCase(Trim(ParamString(Params, 'phylogeny_test', 'None')));
  finally
    Params.Free;
  end;
  Result := Pos('bootstrap', Value) > 0;
end;

function RuntimeTreeBootstrapReps(const Request: TRuntimeRequest): Integer;
var
  Params: TJSONObject;
begin
  Params := TreeParamsObject(Request);
  try
    Result := ParamInt(Params, 'bootstrap_replicates', 500);
  finally
    Params.Free;
  end;
  if Result < 1 then
    Result := 1;
end;

procedure CreateMegaDistanceComputer(const Request: TRuntimeRequest; Names, Seqs: TStringList; IsNucleotide: Boolean;
  out DistPack: TDistPack; out DistComputer: TSeqDistBase; out MappedData: TList;
  out Distances: PDistanceMatrix; out NoOfSites: Integer; out NoOfSeqs: Integer);
var
  I: Integer;
  Freqs: ArrayOfInteger;
begin
  DistPack := nil;
  DistComputer := nil;
  MappedData := nil;
  Distances := nil;

  try
    DistPack := BuildMegaDistancePack(Request, IsNucleotide);
    MappedData := PrepareMegaDistanceMappedData(Request, Names, Seqs, IsNucleotide, NoOfSeqs, NoOfSites);
    Distances := NewDistMatrix(NoOfSeqs, True);
    if IsNucleotide then
      DistComputer := TNucDist.Create
    else
      DistComputer := TAminoDist.Create;

    DistComputer.DistPack := DistPack;
    DistComputer.NoOfSeqs := NoOfSeqs;
    DistComputer.Sequences := MappedData;
    DistComputer.QuickExit := True;
    DistComputer.showInProgressBar := False;
    DistComputer.D := Distances;
    DistComputer.NoOfSites := NoOfSites;

    if IsNucleotide then
    begin
      if not TNucDist(DistComputer).ComputeDistances then
        raise Exception.Create('MEGA nucleotide distance computation failed');
    end
    else if not TAminoDist(DistComputer).ComputeDistances then
      raise Exception.Create('MEGA amino-acid distance computation failed');

    SetLength(Freqs, NoOfSites);
    for I := 0 to NoOfSites - 1 do
      Freqs[I] := 1;
    DistComputer.FreqTable := Freqs;
    DistComputer.FreqTableLen := NoOfSites;
  except
    if Distances <> nil then
      DestroyDistanceMatrix(Distances, NoOfSeqs);
    DistComputer.Free;
    DistPack.Free;
    FreeMegaMappedData(MappedData);
    Distances := nil;
    DistComputer := nil;
    DistPack := nil;
    MappedData := nil;
    raise;
  end;
end;

function RuntimeThreadCount(const Request: TRuntimeRequest; const DefaultValue: Integer): Integer; forward;

procedure RunMegaDistanceBootstrap(const Request: TRuntimeRequest; const Method: String; Names: TStringList;
  DistComputer: TSeqDistBase; Distances: PDistanceMatrix; TreeList: TTreeList);
var
  Info: TAnalysisInfo;
  Progress: TRuntimeProgress;
  BootTree: TTreeData;
  BootThread: TBootstrapCustomTreeThread;
begin
  if Names.Count < 4 then
    raise Exception.Create('At least four taxa are needed for bootstrapping.');
  if TreeList = nil then
    raise Exception.Create('MEGA bootstrap requires a tree list');

  Info := nil;
  Progress := nil;
  BootTree := nil;
  BootThread := nil;
  try
    Info := TAnalysisInfo.Create;
    Info.MyNoOfSeqs := Names.Count;
    Info.MyNoOfSites := DistComputer.NoOfSites;
    Info.DistComputer := DistComputer;
    Info.MyBootPartitionList := TPartitionList.Create(Names.Count, 0, False);

    Progress := TRuntimeProgress.Create(nil);
    Progress.FMAI := Info;

    if SameText(Method, 'minimum_evolution') then
    begin
      if TreeList.NoOfTrees < 1 then
        raise Exception.Create('MEGA minimum-evolution bootstrap requires an initial tree');
      BootThread := TBootstrapMEThread.Create(TreeList, Distances);
    end
    else
    begin
      BootTree := TTreeData.Create(Names.Count, True, True, True);
      if SameText(Method, 'neighbor_joining') then
        BootThread := TBootstrapNJThread.Create(BootTree, Distances)
      else if SameText(Method, 'upgma') then
        BootThread := TBootstrapUPGMAThread.Create(BootTree, Distances)
      else
        raise Exception.Create('unsupported MEGA distance bootstrap method: ' + Method);
      TreeList.Add(BootTree);
      BootTree := nil;
    end;

    BootThread.FreeOnTerminate := False;
    BootThread.NoOfThreads := RuntimeThreadCount(Request, 1);
    BootThread.NoOfBootstraps := RuntimeTreeBootstrapReps(Request);
    BootThread.BootstrapTrees := Info.MyBootPartitionList;
    BootThread.ProgressDlg := Progress;
    BootThread.Info := Info;
    BootThread.DoFullProgress := False;
    AppendRuntimeLog(Request,
      'distance.bootstrap.start method=' + Method +
      ' reps=' + IntToStr(BootThread.NoOfBootstraps) +
      ' threads=' + IntToStr(BootThread.NoOfThreads));
    BootThread.Start;
    WaitForRuntimeThread(BootThread);
    if BootThread.Canceled or (Info.MyBootPartitionList.TotalFrequency < 1) then
      raise Exception.Create('MEGA distance bootstrap did not complete');
    AppendRuntimeLog(Request, 'distance.bootstrap.complete valid_reps=' + IntToStr(Round(Info.MyBootPartitionList.TotalFrequency)));
  finally
    if Assigned(BootThread) then
    begin
      BootThread.ProgressDlg := nil;
      BootThread.Info := nil;
    end;
    BootThread.Free;
    BootTree.Free;
    if Assigned(Info) then
      Info.DistComputer := nil;
    Progress.Free;
    Info.Free;
  end;
end;

function BuildMegaDistanceTreeNewick(const Request: TRuntimeRequest; Names, Seqs: TStringList): String;
var
  IsNucleotide: Boolean;
  Method: String;
  DistPack: TDistPack;
  DistComputer: TSeqDistBase;
  MappedData: TList;
  Distances: PDistanceMatrix;
  Tree: TTreeData;
  TreeList: TTreeList;
  NJComputer: TNJTreeComputer;
  UPGMAComputer: TUPGMATreeComputer;
  MEComputer: TMETreeComputer;
  NoOfSites, NoOfSeqs: Integer;
begin
  if Names.Count < 2 then
    raise Exception.Create('at least two sequences are required for MEGA tree inference');
  DistPack := nil;
  DistComputer := nil;
  MappedData := nil;
  Distances := nil;
  Tree := nil;
  TreeList := nil;
  NJComputer := nil;
  UPGMAComputer := nil;
  MEComputer := nil;
  IsNucleotide := RuntimeTreeUsesNucleotide(Request);
  Method := RuntimeTreeMethod(Request);
  try
    CreateMegaDistanceComputer(Request, Names, Seqs, IsNucleotide, DistPack, DistComputer, MappedData, Distances, NoOfSites, NoOfSeqs);
    TreeList := TTreeList.Create;
    TreeList.OTUNameList.AddStrings(Names);
    if RuntimeTreeBootstrapSelected(Request) then
    begin
      if SameText(Method, 'minimum_evolution') then
      begin
        Tree := TTreeData.Create(Names.Count, True, True, True);
        TreeList.Add(Tree);
        Tree := nil;
      end;
      RunMegaDistanceBootstrap(Request, Method, Names, DistComputer, Distances, TreeList);
    end
    else if SameText(Method, 'neighbor_joining') then
    begin
      Tree := TTreeData.Create(Names.Count, True, False, False);
      NJComputer := TNJTreeComputer.Create(Tree, Distances);
      NJComputer.MakeTree;
      TreeList.Add(Tree);
      Tree := nil;
    end
    else if SameText(Method, 'upgma') then
    begin
      Tree := TTreeData.Create(Names.Count, True, False, False);
      UPGMAComputer := TUPGMATreeComputer.Create(Tree, Distances);
      UPGMAComputer.MakeTree;
      TreeList.Add(Tree);
      Tree := nil;
    end
    else if SameText(Method, 'minimum_evolution') then
    begin
      Tree := TTreeData.Create(Names.Count, True, False, False);
      TreeList.Add(Tree);
      Tree := nil;
      MEComputer := TMETreeComputer.Create(TreeList, Distances);
      MEComputer.UseInitialNJTree := True;
      MEComputer.MaxNoOfTrees := 1;
      MEComputer.Threshold := 1e-10;
      MEComputer.MakeTree;
    end
    else
      raise Exception.Create('unsupported MEGA distance tree method: ' + Request.TreeMethod);

    if TreeList.NoOfTrees < 1 then
      raise Exception.Create('MEGA tree inference returned no trees');
    Result := String(TreeList.OutputNewickTree(0, True, RuntimeTreeBootstrapSelected(Request), 0.0));
    if Trim(Result) = '' then
      raise Exception.Create('MEGA Newick export returned an empty tree');
  finally
    MEComputer.Free;
    UPGMAComputer.Free;
    NJComputer.Free;
    Tree.Free;
    TreeList.Free;
    if Distances <> nil then
      DestroyDistanceMatrix(Distances, Names.Count);
    DistComputer.Free;
    DistPack.Free;
    FreeMegaMappedData(MappedData);
  end;
end;

function RuntimeMLInitialTreeMethod(const Request: TRuntimeRequest): Integer;
var
  Params: TJSONObject;
  Value: String;
begin
  Result := DefaultInitTreeMethod;
  Params := TreeParamsObject(Request);
  try
    Value := LowerCase(Trim(ParamString(Params, 'initial_tree_for_ml', '')));
  finally
    Params.Free;
  end;
  if Value = '' then
    Exit;
  if Pos('use tree from file', Value) > 0 then
    Result := UserProvidedInitTree
  else if Pos('multiple', Value) > 0 then
    Result := MultipleMPTreesMethod
  else if Pos('neighbor', Value) > 0 then
    Result := NJInitTreeMethod
  else if Pos('parsimony', Value) > 0 then
    Result := MPInitTreeMethod
  else if Pos('default', Value) > 0 then
    Result := DefaultInitTreeMethod;
end;

function RuntimeMLInitialTreeFile(const Request: TRuntimeRequest): String;
var
  Params: TJSONObject;
begin
  Params := TreeParamsObject(Request);
  try
    Result := Trim(ParamString(Params, 'initial_tree_file', ''));
  finally
    Params.Free;
  end;
  if Result <> '' then
    Result := ResolveGuideTreeFile(Request, Result);
end;

function RuntimeMLNumberOfInitialTrees(const Request: TRuntimeRequest): Integer;
var
  Params: TJSONObject;
begin
  Params := TreeParamsObject(Request);
  try
    Result := ParamInt(Params, 'number_of_initial_trees', 10);
  finally
    Params.Free;
  end;
end;

function RuntimeMLSearchLevel(const Request: TRuntimeRequest): Integer;
var
  Params: TJSONObject;
  Value: String;
begin
  Result := 1;
  Params := TreeParamsObject(Request);
  try
    Value := LowerCase(Trim(ParamString(Params, 'ml_heuristic_method', '')));
  finally
    Params.Free;
  end;
  if Pos('extensive', Value) > 0 then
    Result := 5
  else if (Pos('spr', Value) > 0) or (Pos('level 3', Value) > 0) then
    Result := 3
  else
    Result := 1;
end;

function RuntimeMLSearchFilter(const Request: TRuntimeRequest): Extended;
var
  Params: TJSONObject;
  Value: String;
begin
  Result := 1.0 + FP_CUTOFF;
  Params := TreeParamsObject(Request);
  try
    Value := LowerCase(Trim(ParamString(Params, 'branch_swap_filter', 'None')));
  finally
    Params.Free;
  end;
  if (Value = '') or (Value = 'none') then
    Result := 1.0 + FP_CUTOFF
  else if Pos('strong', Value) > 0 then
    Result := 0.5
  else if Pos('moderate', Value) > 0 then
    Result := 0.7
  else if Pos('weak', Value) > 0 then
    Result := 0.9
  else
    raise Exception.Create('MEGA maximum-likelihood branch swap filter is not available: ' + Value);
end;

function RuntimeMLGamma(const Request: TRuntimeRequest): Double;
var
  Params: TJSONObject;
  Rates: String;
begin
  Result := 0;
  Params := TreeParamsObject(Request);
  try
    Rates := LowerCase(Trim(ParamString(Params, 'rates_among_sites', 'Uniform Rates')));
    if Pos('gamma distributed', Rates) > 0 then
      Result := ParamFloat(Params, 'gamma_parameter', 1.0);
  finally
    Params.Free;
  end;
end;

function RuntimeMLUseInvar(const Request: TRuntimeRequest): Boolean;
var
  Params: TJSONObject;
  Rates: String;
begin
  Result := False;
  Params := TreeParamsObject(Request);
  try
    Rates := LowerCase(Trim(ParamString(Params, 'rates_among_sites', 'Uniform Rates')));
    Result := Pos('invariant', Rates) > 0;
  finally
    Params.Free;
  end;
end;

function RuntimeMLGammaCategories(const Request: TRuntimeRequest): Integer;
var
  Params: TJSONObject;
begin
  Params := TreeParamsObject(Request);
  try
    Result := ParamInt(Params, 'discrete_gamma_categories', 5);
  finally
    Params.Free;
  end;
  if Result < 1 then
    Result := 1;
end;

function RuntimeThreadCount(const Request: TRuntimeRequest; const DefaultValue: Integer): Integer;
var
  Params: TJSONObject;
begin
  Params := TreeParamsObject(Request);
  try
    Result := ParamInt(Params, 'number_of_threads', DefaultValue);
  finally
    Params.Free;
  end;
  if Result < 1 then
    Result := 1;
end;

function RuntimeMLModel(const Request: TRuntimeRequest; IsNucleotide: Boolean): TGammaRateVariationModel;
var
  Params: TJSONObject;
  Model: String;
  Gamma: Double;
  UseInvar: Boolean;
  GammaCats: Integer;
begin
  Params := TreeParamsObject(Request);
  try
    Model := LowerCase(Trim(ParamString(Params, 'model_method', 'Jones-Taylor-Thornton (JTT) model')));
  finally
    Params.Free;
  end;
  Gamma := RuntimeMLGamma(Request);
  UseInvar := RuntimeMLUseInvar(Request);
  GammaCats := RuntimeMLGammaCategories(Request);

  if IsNucleotide then
  begin
    if Pos('jukes', Model) > 0 then
      Exit(TJCModel.Create(Gamma, UseInvar, GammaCats));
    if Pos('kimura', Model) > 0 then
      Exit(TK2Model.Create(Gamma, UseInvar, GammaCats));
    if Pos('tamura 3', Model) > 0 then
      Exit(TT3Model.Create(Gamma, UseInvar, GammaCats));
    if Pos('hasegawa', Model) > 0 then
      Exit(THKYModel.Create(Gamma, UseInvar, GammaCats));
    if (Model = '') or (Pos('tamura-nei', Model) > 0) then
      Exit(TTN93Model.Create(Gamma, UseInvar, GammaCats));
    if Pos('general time reversible', Model) > 0 then
      Exit(TGTRModel.Create(Gamma, UseInvar, GammaCats));
    raise Exception.Create('MEGA maximum-likelihood nucleotide model is not available in this runtime request: ' + Model);
  end;

  if (Model = '') or (Pos('jones', Model) > 0) or (Pos('jtt', Model) > 0) then
    Exit(TJTTModel.Create(Gamma, UseInvar, Pos('+f', Model) > 0, GammaCats));
  if Pos('dayhoff', Model) > 0 then
    Exit(TDayhoffModel.Create(Gamma, UseInvar, Pos('+f', Model) > 0, GammaCats));
  if Pos('wag', Model) > 0 then
    Exit(TWAGModel.Create(Gamma, UseInvar, Pos('+f', Model) > 0, GammaCats));
  if Pos('lg', Model) > 0 then
    Exit(TLGModel.Create(Gamma, UseInvar, Pos('+f', Model) > 0, GammaCats));
  if Pos('mtrev', Model) > 0 then
    Exit(TmtREV24Model.Create(Gamma, UseInvar, Pos('+f', Model) > 0, GammaCats));
  if Pos('cprev', Model) > 0 then
    Exit(TcpREVModel.Create(Gamma, UseInvar, Pos('+f', Model) > 0, GammaCats));
  if Pos('rtrev', Model) > 0 then
    Exit(TrtREVModel.Create(Gamma, UseInvar, Pos('+f', Model) > 0, GammaCats));
  if Pos('poisson', Model) > 0 then
    Exit(TPoissonModel.Create(Gamma, UseInvar, False, GammaCats));
  if Pos('equal input', Model) > 0 then
    Exit(TPoissonModel.Create(Gamma, UseInvar, True, GammaCats));
  raise Exception.Create('MEGA maximum-likelihood amino-acid model is not available in this runtime request: ' + Model);
end;

function RuntimeMLDistType(const Request: TRuntimeRequest; IsNucleotide: Boolean): TDistType;
var
  Params: TJSONObject;
  Model: String;
begin
  Params := TreeParamsObject(Request);
  try
    Model := LowerCase(Trim(ParamString(Params, 'model_method', 'Jones-Taylor-Thornton (JTT) model')));
  finally
    Params.Free;
  end;

  if IsNucleotide then
  begin
    if Pos('jukes', Model) > 0 then
      Exit(gdJukesCantor);
    if Pos('kimura', Model) > 0 then
      Exit(gdKimura2para);
    if Pos('tamura 3', Model) > 0 then
      Exit(gdTamura);
    if Pos('hasegawa', Model) > 0 then
      Exit(gdHKY);
    if (Model = '') or (Pos('tamura-nei', Model) > 0) then
      Exit(gdTamuraNei);
    if Pos('general time reversible', Model) > 0 then
      Exit(gdREV);
  end
  else
  begin
    if Pos('jones', Model) > 0 then
    begin
      if Pos('+f', Model) > 0 then
        Exit(gdJonesPi);
      Exit(gdJones);
    end;
    if Pos('dayhoff', Model) > 0 then
    begin
      if Pos('+f', Model) > 0 then
        Exit(gdDayhoffPi);
      Exit(gdDayhoff);
    end;
    if Pos('wag', Model) > 0 then
    begin
      if Pos('+f', Model) > 0 then
        Exit(gdWAGPi);
      Exit(gdWAG);
    end;
    if Pos('lg', Model) > 0 then
    begin
      if Pos('+f', Model) > 0 then
        Exit(gdLGPi);
      Exit(gdLG);
    end;
    if Pos('mtrev', Model) > 0 then
    begin
      if Pos('+f', Model) > 0 then
        Exit(gdMtRev24Pi);
      Exit(gdMtRev24);
    end;
    if Pos('cprev', Model) > 0 then
    begin
      if Pos('+f', Model) > 0 then
        Exit(gdCpRevPi);
      Exit(gdCpRev);
    end;
    if Pos('rtrev', Model) > 0 then
    begin
      if Pos('+f', Model) > 0 then
        Exit(gdRtRevPi);
      Exit(gdRtRev);
    end;
    if Pos('poisson', Model) > 0 then
      Exit(gdPoisson);
    if Pos('equal input', Model) > 0 then
      Exit(gdEqualInput);
  end;
  raise Exception.Create('MEGA maximum-likelihood model is not available in this runtime request: ' + Model);
end;

function BuildMegaMLDistPack(const Request: TRuntimeRequest; IsNucleotide: Boolean): TDistPack;
var
  Params: TJSONObject;
  Rates: String;
begin
  Result := TDistPack.Create;
  try
    if IsNucleotide then
      Result.AddType(gdOneNuc)
    else
      Result.AddType(gdAmino);
    Result.AddType(RuntimeMLDistType(Request, IsNucleotide));
    Params := TreeParamsObject(Request);
    try
      Rates := LowerCase(Trim(ParamString(Params, 'rates_among_sites', 'Uniform Rates')));
      if Pos('gamma distributed', Rates) > 0 then
      begin
        Result.AddType(gdGamma);
      end;
      if Pos('invariant', Rates) > 0 then
        Result.AddType(gdInvar);
    finally
      Params.Free;
    end;
  except
    Result.Free;
    raise;
  end;
end;

function BuildMegaMaximumLikelihoodNewick(const Request: TRuntimeRequest; Names, Seqs: TStringList): String;
var
  Analyzer: TMLTreeAnalyzer;
  Model: TGammaRateVariationModel;
  SeqCopy, NameCopy: TStringList;
  Tree: TTreeData;
  TreeList: TTreeList;
  Progress: TRuntimeProgress;
  SearchThread: TPHgoMLTreeSearchThread;
  MultiStartThread: TMLTreeSearchThread;
  BootThread: TBootstrapMLThread;
  Info: TAnalysisInfo;
  MLDistPack: TDistPack;
  IsNucleotide: Boolean;
  InitialTreeMethod: Integer;
  InitialTreeCount: Integer;
  InitialTreePath: String;
  InitialTreeList: TTreeList;
  InitialTreeData: TTreeData;
begin
  if Names.Count < 2 then
    raise Exception.Create('at least two sequences are required for MEGA maximum-likelihood tree inference');

  Analyzer := nil;
  Model := nil;
  SeqCopy := nil;
  NameCopy := nil;
  Tree := nil;
  TreeList := nil;
  Progress := nil;
  SearchThread := nil;
  MultiStartThread := nil;
  BootThread := nil;
  Info := nil;
  MLDistPack := nil;
  InitialTreeList := nil;
  InitialTreeData := nil;
  IsNucleotide := RuntimeTreeUsesNucleotide(Request);
  try
    PrepareMegaMLSequenceData(Request, Names, Seqs, IsNucleotide, NameCopy, SeqCopy);
    InitialTreeMethod := RuntimeMLInitialTreeMethod(Request);
    if (not RuntimeTreeBootstrapSelected(Request)) and (InitialTreeMethod = MultipleMPTreesMethod) then
    begin
      Info := TAnalysisInfo.Create;
      Info.MyUsrOperation := dtdoMLTree;
      Info.MyNoOfSeqs := NameCopy.Count;
      Info.MyNoOfSites := MegaAlignedSiteCount(SeqCopy);
      Info.MyOtuNames := NameCopy;
      NameCopy := nil;
      Info.MySeqStrings := SeqCopy;
      SeqCopy := nil;
      MLDistPack := BuildMegaMLDistPack(Request, IsNucleotide);
      Info.MyDistPack := MLDistPack;
      MLDistPack := nil;
      Info.MyTreePack := TTreePack.Create;
      Info.MyTreePack.AddType(ttML);
      Info.MyTreePack.AddType(ttInferTree);
      if RuntimeMLSearchLevel(Request) = 3 then
        Info.MyTreePack.AddType(ttSPRFast)
      else if RuntimeMLSearchLevel(Request) = 5 then
        Info.MyTreePack.AddType(ttSPRExtensive)
      else
        Info.MyTreePack.AddType(ttNNI);
      Info.MyInitialTreeMethod := InitialTreeMethod;
      InitialTreeCount := RuntimeMLNumberOfInitialTrees(Request);
      Info.NumStartingMpTrees := InitialTreeCount;
      Info.MyProcessPack := TProcessPack.Create;
      Info.MyProcessPack.TextualSettingsList.Values['No. of Initial Trees'] := IntToStr(InitialTreeCount);
      Info.MyNumThreadsToUse := RuntimeThreadCount(Request, 1);
      Info.MyMLSearchFilter := RuntimeMLSearchFilter(Request);
      Progress := TRuntimeProgress.Create(nil);
      Progress.FMAI := Info;
      Info.ARP := Progress;
      MultiStartThread := TMLTreeSearchThread.Create(nil);
      MultiStartThread.FreeOnTerminate := False;
      MultiStartThread.ProgressDlg := Progress;
      MultiStartThread.Info := Info;
      MultiStartThread.ShowProgress := True;
      AppendRuntimeLog(Request,
        'maximum_likelihood.multi_start.start initial_trees=' + IntToStr(InitialTreeCount) +
        ' threads=' + IntToStr(Info.MyNumThreadsToUse));
      MultiStartThread.Start;
      WaitForRuntimeThread(MultiStartThread);
      if MultiStartThread.Canceled then
        raise Exception.Create('MEGA maximum-likelihood multiple-initial-tree search was cancelled');
      if (Info.MyOriTreeList = nil) or (Info.MyOriTreeList.NoOfTrees < 1) then
        raise Exception.Create('MEGA maximum-likelihood multiple-initial-tree search returned no tree');
      Tree := TTreeData.Create(Info.MyOriTreeList[0].NoOfOTUs, Info.MyOriTreeList[0].isBLen, Info.MyOriTreeList[0].isSE, Info.MyOriTreeList[0].isStats);
      Tree.Assign(Info.MyOriTreeList[0]);
      TreeList := TTreeList.Create;
      TreeList.OTUNameList.Assign(Info.MyOtuNames);
      TreeList.Add(Tree);
      Tree := nil;
      Result := String(TreeList.OutputNewickTree(0, True, False, 0.0));
      if Trim(Result) = '' then
        raise Exception.Create('MEGA maximum-likelihood multiple-initial-tree Newick export returned an empty tree');
      AppendRuntimeLog(Request, 'maximum_likelihood.multi_start complete=true');
      Exit;
    end;
    Model := RuntimeMLModel(Request, IsNucleotide);
    Model.SetParamsFromSeqs(SeqCopy);
    Model.NoOfThreads := RuntimeThreadCount(Request, 1);
    AppendRuntimeLog(Request, 'maximum_likelihood.model created=true nucleotide=' + BoolToStr(IsNucleotide, True));
    Analyzer := TMLTreeAnalyzer.Create(SeqCopy, nil, Model);
    SeqCopy := nil;
    Model := nil;
    Analyzer.SeqNames := NameCopy;
    NameCopy := nil;
    Progress := TRuntimeProgress.Create(nil);
    Analyzer.RuntimeProgress := Progress;
    Analyzer.SearchLevel := RuntimeMLSearchLevel(Request);
    Analyzer.SearchFilter := RuntimeMLSearchFilter(Request);
    InitialTreeMethod := RuntimeMLInitialTreeMethod(Request);
    Analyzer.InitialTreeOption := InitialTreeMethod;
    if InitialTreeMethod = UserProvidedInitTree then
    begin
      InitialTreePath := RuntimeMLInitialTreeFile(Request);
      if Trim(InitialTreePath) = '' then
        raise Exception.Create('MEGA maximum-likelihood initial tree file was not provided');
      InitialTreeList := TTreeList.Create;
      if not InitialTreeList.ImportFromNewickFile(InitialTreePath, Analyzer.SeqNames, False) then
        raise Exception.Create('MEGA could not import the maximum-likelihood initial tree: ' + InitialTreePath);
      if InitialTreeList.NoOfTrees < 1 then
        raise Exception.Create('MEGA maximum-likelihood initial tree file did not contain a tree');
      InitialTreeData := TTreeData.Create(InitialTreeList[0].NoOfOTUs, InitialTreeList[0].isBLen, InitialTreeList[0].isSE, InitialTreeList[0].isStats);
      InitialTreeData.Assign(InitialTreeList[0]);
      Analyzer.SetTreeData(InitialTreeData);
      InitialTreeData := nil;
    end;
    Analyzer.NoOfThreadsToUse := RuntimeThreadCount(Request, 1);
    AppendRuntimeLog(Request,
      'maximum_likelihood.start search_level=' + IntToStr(Analyzer.SearchLevel) +
      ' search_filter=' + FloatToStrF(Analyzer.SearchFilter, ffFixed, 8, 5) +
      ' initial_tree=' + IntToStr(Analyzer.InitialTreeOption) +
      ' threads=' + IntToStr(Analyzer.NoOfThreadsToUse));

    if RuntimeTreeBootstrapSelected(Request) then
    begin
      if Analyzer.SeqNames.Count < 4 then
        raise Exception.Create('At least four taxa are needed for bootstrapping.');
      Info := TAnalysisInfo.Create;
      Info.MyNoOfSeqs := Analyzer.SeqNames.Count;
      Info.MyNoOfSites := MegaAlignedSiteCount(Analyzer.SeqData);
      Info.MyOtuNames := TStringList.Create;
      Info.MyOtuNames.Assign(Analyzer.SeqNames);
      Info.MySeqStrings := TStringList.Create;
      Info.MySeqStrings.Assign(Analyzer.SeqData);
      MLDistPack := BuildMegaMLDistPack(Request, IsNucleotide);
      Info.MyDistPack := MLDistPack;
      MLDistPack := nil;
      Info.MyTreePack := TTreePack.Create;
      Info.MyTreePack.AddType(ttML);
      Info.MyTreePack.AddType(ttInferTree);
      Info.MyTreePack.AddType(ttBootstrap);
      if Analyzer.SearchLevel = 3 then
        Info.MyTreePack.AddType(ttSPRFast)
      else if Analyzer.SearchLevel = 5 then
        Info.MyTreePack.AddType(ttSPRExtensive)
      else
        Info.MyTreePack.AddType(ttNNI);
      Info.MyInitialTreeMethod := InitialTreeMethod;
      Info.MyNumThreadsToUse := RuntimeThreadCount(Request, 1);
      Info.MyMLSearchFilter := Analyzer.SearchFilter;
      Info.MyBootReps := RuntimeTreeBootstrapReps(Request);
      Info.MyBootPartitionList := TPartitionList.Create(Analyzer.SeqNames.Count, 0, False);
      Info.ARP := Progress;
      Progress.FMAI := Info;
      Info.MyTreePack.BootReps := Info.MyBootReps;
      BootThread := TBootstrapMLThread.Create(nil);
      BootThread.FreeOnTerminate := False;
      BootThread.ProgressDlg := Progress;
      BootThread.NoOfReplication := Info.MyBootReps;
      BootThread.BootstrapTrees := Info.MyBootPartitionList;
      BootThread.NoOfThreads := Info.MyNumThreadsToUse;
      BootThread.ShowProgress := True;
      AppendRuntimeLog(Request,
        'maximum_likelihood.bootstrap.start reps=' + IntToStr(BootThread.NoOfReplication) +
        ' threads=' + IntToStr(BootThread.NoOfThreads));
      BootThread.Start;
      WaitForRuntimeThread(BootThread);
      if BootThread.Canceled or (not BootThread.IsSuccess) then
      begin
        AppendRuntimeLog(Request, 'maximum_likelihood.bootstrap.error ' + StringReplace(BootThread.LogText, LineEnding, ' | ', [rfReplaceAll]));
        raise Exception.Create('MEGA maximum-likelihood bootstrap did not complete');
      end;
      if (Info.MyOriTreeList = nil) or (Info.MyOriTreeList.NoOfTrees < 1) then
        raise Exception.Create('MEGA maximum-likelihood bootstrap returned no original tree');
      Tree := TTreeData.Create(Info.MyOriTreeList[0].NoOfOTUs, Info.MyOriTreeList[0].isBLen, Info.MyOriTreeList[0].isSE, Info.MyOriTreeList[0].isStats);
      Tree.Assign(Info.MyOriTreeList[0]);
      BootThread := nil;
      Progress.FMAI := nil;
      Info.MySeqStrings := nil;
      AppendRuntimeLog(Request, 'maximum_likelihood.bootstrap complete=true');
    end
    else
    begin
      if InitialTreeMethod = MultipleMPTreesMethod then
      begin
        Info := TAnalysisInfo.Create;
        Info.MyUsrOperation := dtdoMLTree;
        Info.MyNoOfSeqs := Analyzer.SeqNames.Count;
        Info.MyNoOfSites := MegaAlignedSiteCount(Analyzer.SeqData);
        Info.MyOtuNames := TStringList.Create;
        Info.MyOtuNames.Assign(Analyzer.SeqNames);
        Info.MySeqStrings := TStringList.Create;
        Info.MySeqStrings.Assign(Analyzer.SeqData);
        MLDistPack := BuildMegaMLDistPack(Request, IsNucleotide);
        Info.MyDistPack := MLDistPack;
        MLDistPack := nil;
        Info.MyTreePack := TTreePack.Create;
        Info.MyTreePack.AddType(ttML);
        Info.MyTreePack.AddType(ttInferTree);
        if Analyzer.SearchLevel = 3 then
          Info.MyTreePack.AddType(ttSPRFast)
        else if Analyzer.SearchLevel = 5 then
          Info.MyTreePack.AddType(ttSPRExtensive)
        else
          Info.MyTreePack.AddType(ttNNI);
        Info.MyInitialTreeMethod := InitialTreeMethod;
        Info.NumStartingMpTrees := RuntimeMLNumberOfInitialTrees(Request);
        Info.MyNumThreadsToUse := RuntimeThreadCount(Request, 1);
        Info.MyMLSearchFilter := Analyzer.SearchFilter;
        Info.ARP := Progress;
        Progress.FMAI := Info;
        MultiStartThread := TMLTreeSearchThread.Create(nil);
        MultiStartThread.FreeOnTerminate := False;
        MultiStartThread.ProgressDlg := Progress;
        MultiStartThread.Info := Info;
        MultiStartThread.ShowProgress := True;
        AppendRuntimeLog(Request,
          'maximum_likelihood.multi_start.start initial_trees=' + IntToStr(Info.NumStartingMpTrees) +
          ' threads=' + IntToStr(Info.MyNumThreadsToUse));
        MultiStartThread.Start;
        WaitForRuntimeThread(MultiStartThread);
        if MultiStartThread.Canceled then
          raise Exception.Create('MEGA maximum-likelihood multiple-initial-tree search was cancelled');
        if (Info.MyOriTreeList = nil) or (Info.MyOriTreeList.NoOfTrees < 1) then
          raise Exception.Create('MEGA maximum-likelihood multiple-initial-tree search returned no tree');
        Tree := TTreeData.Create(Info.MyOriTreeList[0].NoOfOTUs, Info.MyOriTreeList[0].isBLen, Info.MyOriTreeList[0].isSE, Info.MyOriTreeList[0].isStats);
        Tree.Assign(Info.MyOriTreeList[0]);
        AppendRuntimeLog(Request, 'maximum_likelihood.multi_start complete=true');
      end
      else
      begin
        SearchThread := TPHgoMLTreeSearchThread.Create(Analyzer);
        SearchThread.ProgressDlg := Progress;
        SearchThread.IsBootstrapReplicate := False;
        if not SearchThread.RunPHgoSearch(Request, InitialTreeMethod <> UserProvidedInitTree) then
          raise Exception.Create('MEGA maximum-likelihood tree search did not complete');
        SearchThread.MLTreeAnalyzer := nil;
        SearchThread := nil;
        AppendRuntimeLog(Request, 'maximum_likelihood.search complete=true');

        Tree := TTreeData.Create(Analyzer.SeqNames.Count, True, True, False);
        Analyzer.GetTreeData(Tree);
      end;
    end;
    TreeList := TTreeList.Create;
    TreeList.OTUNameList.AddStrings(Analyzer.SeqNames);
    TreeList.Add(Tree);
    Tree := nil;
    Result := String(TreeList.OutputNewickTree(0, True, RuntimeTreeBootstrapSelected(Request), 0.0));
    if Trim(Result) = '' then
      raise Exception.Create('MEGA maximum-likelihood Newick export returned an empty tree');
  finally
    TreeList.Free;
    Tree.Free;
    InitialTreeData.Free;
    InitialTreeList.Free;
    if Assigned(SearchThread) then
      SearchThread.MLTreeAnalyzer := nil;
    SearchThread.Free;
    MultiStartThread.Free;
    if Assigned(BootThread) then
      BootThread.MLTreeAnalyzer := nil;
    BootThread.Free;
    if Assigned(Info) then
      Info.MySeqStrings := nil;
    if Assigned(Progress) then
      Progress.FMAI := nil;
    Info.Free;
    MLDistPack.Free;
    Analyzer.Free;
    Progress.Free;
    NameCopy.Free;
    SeqCopy.Free;
    Model.Free;
  end;
end;

function RuntimeMPSearchMethod(const Request: TRuntimeRequest): TSearchMethod;
var
  Params: TJSONObject;
  Value: String;
begin
  Result := SPR;
  Params := TreeParamsObject(Request);
  try
    Value := LowerCase(Trim(ParamString(Params, 'mp_search_method', '')));
  finally
    Params.Free;
  end;
  if Pos('tbr', Value) > 0 then
    Result := TBR
  else
    Result := SPR;
end;

function RuntimeMPSearchMethodText(const Request: TRuntimeRequest): String;
var
  Params: TJSONObject;
begin
  Params := TreeParamsObject(Request);
  try
    Result := LowerCase(Trim(ParamString(Params, 'mp_search_method', 'Subtree-Pruning-Regrafting (SPR)')));
  finally
    Params.Free;
  end;
end;

function RuntimeMPIntParam(const Request: TRuntimeRequest; const Name: String; const DefaultValue: Integer): Integer;
var
  Params: TJSONObject;
begin
  Params := TreeParamsObject(Request);
  try
    Result := ParamInt(Params, Name, DefaultValue);
  finally
    Params.Free;
  end;
end;

function BuildMegaMaximumParsimonyNewick(const Request: TRuntimeRequest; Names, Seqs: TStringList): String;
var
  MPTree: TMPTree;
  TreeList: TTreeList;
  MappedData: TList;
  BootThread: TBootstrapMPTreeSearchThread;
  BranchThread: TBranchBoundSearchThread;
  MiniMiniThread: TMiniMini_CNISearchThread;
  BootBranchThread: TBootstrapBranchBoundSearchThread;
  BootMiniMiniThread: TBootstrapMiniMini_CNISearchThread;
  Progress: TRuntimeProgress;
  Info: TAnalysisInfo;
  IsNucleotide: Boolean;
  MethodText: String;
  Bits: Integer;
  NoOfSites, NoOfSeqs: Integer;
begin
  if Names.Count < 2 then
    raise Exception.Create('at least two sequences are required for MEGA maximum-parsimony tree inference');

  MPTree := nil;
  TreeList := nil;
  MappedData := nil;
  BootThread := nil;
  BranchThread := nil;
  MiniMiniThread := nil;
  BootBranchThread := nil;
  BootMiniMiniThread := nil;
  Progress := nil;
  Info := nil;
  IsNucleotide := RuntimeTreeUsesNucleotide(Request);
  if IsNucleotide then
    Bits := 8
  else
    Bits := 32;
  try
    TreeList := TTreeList.Create;
    TreeList.OTUNameList.AddStrings(Names);
    MethodText := RuntimeMPSearchMethodText(Request);
    if RuntimeTreeBootstrapSelected(Request) then
    begin
      if Names.Count < 4 then
        raise Exception.Create('At least four taxa are needed for bootstrapping.');
      MappedData := PrepareMegaParsimonyMappedData(Request, Names, Seqs, IsNucleotide, NoOfSeqs, NoOfSites);
      Info := TAnalysisInfo.Create;
      Info.MyNoOfSeqs := NoOfSeqs;
      Info.MyNoOfSites := NoOfSites;
      Info.MyBootReps := RuntimeTreeBootstrapReps(Request);
      Info.MyBootPartitionList := TPartitionList.Create(NoOfSeqs, 0, False);
      Progress := TRuntimeProgress.Create(nil);
      Progress.FMAI := Info;
      if (Pos('max-mini', MethodText) > 0) or (Pos('branch-&-bound', MethodText) > 0) then
      begin
        BootBranchThread := TBootstrapBranchBoundSearchThread.Create(TreeList, MappedData, NoOfSites, Bits, Info.MyBootReps);
        BootBranchThread.FreeOnTerminate := False;
        BootBranchThread.MaxNoOfTrees := RuntimeMPIntParam(Request, 'max_trees_to_retain', 100);
        BootBranchThread.BootstrapTrees := Info.MyBootPartitionList;
        BootBranchThread.ProgressDlg := Progress;
        BootBranchThread.ShowProgress := True;
        BootBranchThread.Info := Info;
        AppendRuntimeLog(Request,
          'maximum_parsimony.bootstrap.start method=max-mini reps=' + IntToStr(Info.MyBootReps));
        BootBranchThread.Start;
        WaitForRuntimeThread(BootBranchThread);
        if (BootBranchThread.MyExceptionName <> 'none') or (Info.MyBootPartitionList.TotalFrequency < 1) then
          raise Exception.Create('MEGA maximum-parsimony Max-mini bootstrap did not complete: ' + BootBranchThread.MyExceptionMessage);
      end
      else if Pos('min-mini', MethodText) > 0 then
      begin
        BootMiniMiniThread := TBootstrapMiniMini_CNISearchThread.Create(TreeList, MappedData, NoOfSites, Bits,
          RuntimeMPIntParam(Request, 'mp_search_level', 1), RuntimeMPIntParam(Request, 'mp_search_level', 1), Info.MyBootReps);
        BootMiniMiniThread.FreeOnTerminate := False;
        BootMiniMiniThread.NoOfThreads := RuntimeThreadCount(Request, 1);
        BootMiniMiniThread.MaxNoOfTrees := RuntimeMPIntParam(Request, 'max_trees_to_retain', 100);
        BootMiniMiniThread.BootstrapTrees := Info.MyBootPartitionList;
        BootMiniMiniThread.ProgressDlg := Progress;
        BootMiniMiniThread.ShowProgress := True;
        BootMiniMiniThread.Info := Info;
        AppendRuntimeLog(Request,
          'maximum_parsimony.bootstrap.start method=min-mini reps=' + IntToStr(Info.MyBootReps) +
          ' threads=' + IntToStr(BootMiniMiniThread.NoOfThreads));
        BootMiniMiniThread.Start;
        WaitForRuntimeThread(BootMiniMiniThread);
        if (BootMiniMiniThread.MyExceptionName <> 'none') or (Info.MyBootPartitionList.TotalFrequency < 1) then
          raise Exception.Create('MEGA maximum-parsimony Min-Mini bootstrap did not complete: ' + BootMiniMiniThread.MyExceptionMessage);
      end
      else
      begin
        BootThread := TBootstrapMPTreeSearchThread.Create(TreeList, MappedData, NoOfSites, Bits, RuntimeThreadCount(Request, 1));
        BootThread.FreeOnTerminate := False;
        BootThread.BootstrapTrees := Info.MyBootPartitionList;
        BootThread.NoOfBootstraps := Info.MyBootReps;
        BootThread.SearchMethod := RuntimeMPSearchMethod(Request);
        BootThread.NoOfInitTrees := RuntimeMPIntParam(Request, 'initial_trees_random_addition', 10);
        BootThread.SearchLevel := RuntimeMPIntParam(Request, 'mp_search_level', 1);
        BootThread.MaxNoOfTrees := RuntimeMPIntParam(Request, 'max_trees_to_retain', 100);
        BootThread.ProgressDlg := Progress;
        BootThread.ShowProgress := True;
        BootThread.Info := Info;
        AppendRuntimeLog(Request,
          'maximum_parsimony.bootstrap.start method=spr_tbr reps=' + IntToStr(BootThread.NoOfBootstraps) +
          ' threads=' + IntToStr(BootThread.NoOfThreads));
        BootThread.Start;
        WaitForRuntimeThread(BootThread);
        if BootThread.Canceled or (Info.MyBootPartitionList.TotalFrequency < 1) then
          raise Exception.Create('MEGA maximum-parsimony bootstrap did not complete');
      end;
      AppendRuntimeLog(Request, 'maximum_parsimony.bootstrap complete=true');
    end
    else
    begin
      MappedData := PrepareMegaParsimonyMappedData(Request, Names, Seqs, IsNucleotide, NoOfSeqs, NoOfSites);
      if (Pos('max-mini', MethodText) > 0) or (Pos('branch-&-bound', MethodText) > 0) then
      begin
        BranchThread := TBranchBoundSearchThread.Create(TreeList, MappedData, NoOfSites, Bits);
        BranchThread.FreeOnTerminate := False;
        BranchThread.MaxNoOfTrees := RuntimeMPIntParam(Request, 'max_trees_to_retain', 100);
        BranchThread.ShowProgress := False;
        BranchThread.Start;
        WaitForRuntimeThread(BranchThread);
        if BranchThread.MyExceptionName <> 'none' then
          raise Exception.Create('MEGA maximum-parsimony Max-mini search did not complete: ' + BranchThread.MyExceptionMessage);
      end
      else if Pos('min-mini', MethodText) > 0 then
      begin
        MiniMiniThread := TMiniMini_CNISearchThread.Create(TreeList, MappedData, NoOfSites, Bits,
          RuntimeMPIntParam(Request, 'mp_search_level', 1), RuntimeMPIntParam(Request, 'mp_search_level', 1));
        MiniMiniThread.FreeOnTerminate := False;
        MiniMiniThread.MaxNoOfTrees := RuntimeMPIntParam(Request, 'max_trees_to_retain', 100);
        MiniMiniThread.ShowProgress := False;
        MiniMiniThread.Start;
        WaitForRuntimeThread(MiniMiniThread);
        if MiniMiniThread.MyExceptionName <> 'none' then
          raise Exception.Create('MEGA maximum-parsimony Min-Mini search did not complete: ' + MiniMiniThread.MyExceptionMessage);
      end
      else
      begin
        MPTree := TMPTree.Create(MappedData, NoOfSites, Bits);
        MPTree.SearchMethod := RuntimeMPSearchMethod(Request);
        MPTree.NoOfInitTrees := RuntimeMPIntParam(Request, 'initial_trees_random_addition', 10);
        MPTree.SearchLevel := RuntimeMPIntParam(Request, 'mp_search_level', 1);
        MPTree.MaxNoOfTrees := RuntimeMPIntParam(Request, 'max_trees_to_retain', 100);
        if not MPTree.SearchMPTree then
          raise Exception.Create('MEGA maximum-parsimony tree search did not complete');
        MPTree.GetTreeList(TreeList);
      end;
    end;
    if TreeList.NoOfTrees < 1 then
      raise Exception.Create('MEGA maximum-parsimony tree inference returned no trees');
    Result := String(TreeList.OutputNewickTree(0, True, RuntimeTreeBootstrapSelected(Request), 0.0));
    if Trim(Result) = '' then
      raise Exception.Create('MEGA maximum-parsimony Newick export returned an empty tree');
  finally
    if Assigned(BootThread) then
    begin
      BootThread.ProgressDlg := nil;
      BootThread.Info := nil;
    end;
    BootThread.Free;
    BranchThread.Free;
    MiniMiniThread.Free;
    if Assigned(BootBranchThread) then
    begin
      BootBranchThread.ProgressDlg := nil;
      BootBranchThread.Info := nil;
    end;
    BootBranchThread.Free;
    if Assigned(BootMiniMiniThread) then
    begin
      BootMiniMiniThread.ProgressDlg := nil;
      BootMiniMiniThread.Info := nil;
    end;
    BootMiniMiniThread.Free;
    if Assigned(Progress) then
      Progress.FMAI := nil;
    Progress.Free;
    Info.Free;
    FreeMegaMappedData(MappedData);
    TreeList.Free;
    MPTree.Free;
  end;
end;

function BuildMegaTreeNewick(const Request: TRuntimeRequest; Names, Seqs: TStringList): String;
var
  Method: String;
begin
  Method := RuntimeTreeMethod(Request);
  if SameText(Method, 'maximum_likelihood') then
    Exit(BuildMegaMaximumLikelihoodNewick(Request, Names, Seqs));
  if SameText(Method, 'maximum_parsimony') then
    Exit(BuildMegaMaximumParsimonyNewick(Request, Names, Seqs));
  Result := BuildMegaDistanceTreeNewick(Request, Names, Seqs);
end;

function RuntimePathsJSON(const Paths: TRuntimePaths): TJSONObject;
begin
  Result := TJSONObject.Create;
  Result.Add('base_dir', Paths.BaseDir);
  Result.Add('input_fasta', Paths.InputFasta);
  Result.Add('metadata_json', Paths.MetadataJson);
  Result.Add('aligned_fasta', Paths.AlignedFasta);
  Result.Add('newick', Paths.Newick);
  Result.Add('summary', Paths.Summary);
  Result.Add('runtime_log', Paths.RuntimeLog);
end;

function SkippedRecordsJSON(const Skipped: TSkippedRuntimeRecords): TJSONArray;
var
  I: Integer;
  Obj: TJSONObject;
begin
  Result := TJSONArray.Create;
  for I := 0 to Length(Skipped) - 1 do
  begin
    Obj := TJSONObject.Create;
    Obj.Add('taxon_id', Skipped[I].TaxonID);
    Obj.Add('item_title', Skipped[I].ItemTitle);
    Obj.Add('row_index', Skipped[I].RowIndex);
    Obj.Add('reason', Skipped[I].Reason);
    Result.Add(Obj);
  end;
end;

procedure WriteResponse(const Request: TRuntimeRequest; const ErrorText: String; const Skipped: TSkippedRuntimeRecords);
var
  Response: TJSONObject;
  ResponsePath: String;
begin
  ResponsePath := IncludeTrailingPathDelimiter(Request.Artifacts.BaseDir) + 'runtime-response.json';
  Response := TJSONObject.Create;
  try
    Response.Add('schema_version', 1);
    Response.Add('runtime', RuntimeName);
    Response.Add('completed_at', RuntimeTimestamp(Now));
    Response.Add('artifacts', RuntimePathsJSON(Request.Artifacts));
    if Length(Skipped) > 0 then
      Response.Add('skipped_records', SkippedRecordsJSON(Skipped));
    if Trim(ErrorText) <> '' then
      Response.Add('error_text', ErrorText);
    SaveTextFile(ResponsePath, Response.FormatJSON([], 2));
  finally
    Response.Free;
  end;
end;

procedure RunRequest(const RequestPath: String);
var
  Request: TRuntimeRequest;
  Names, Seqs: TStringList;
  Skipped: TSkippedRuntimeRecords;
  Newick: String;
begin
  Names := TStringList.Create;
  Seqs := TStringList.Create;
  Skipped := nil;
  SetLength(Skipped, 0);
  try
      Request := ParseRequest(RequestPath);
      if Request.Artifacts.BaseDir = '' then
        Request.Artifacts.BaseDir := ExtractFileDir(RequestPath);
    ForceDirectories(Request.Artifacts.BaseDir);
    try
      SaveTextFile(Request.Artifacts.RuntimeLog, RuntimeName + ' started' + LineEnding);
      ParseFasta(Request.InputFasta, Names, Seqs);
      AppendRuntimeLog(Request, 'fasta.parsed taxa=' + IntToStr(Names.Count));
      if Names.Count <> Seqs.Count then
        raise Exception.Create('invalid FASTA input');
      if Names.Count < 2 then
        raise Exception.Create('at least two FASTA records are required');

      RunAlignment(Request, Names, Seqs);
      SaveTextFile(Request.Artifacts.AlignedFasta, FastaFromLists(Names, Seqs));

      Newick := BuildMegaTreeNewick(Request, Names, Seqs);
      SaveTextFile(Request.Artifacts.Newick, Trim(Newick) + LineEnding);
      SaveTextFile(Request.Artifacts.Summary,
        'runtime=' + RuntimeName + LineEnding +
        'alignment_method=' + Request.AlignmentMethod + LineEnding +
        'tree_method=' + Request.TreeMethod + LineEnding +
        'taxa=' + IntToStr(Names.Count) + LineEnding +
        'skipped_records=' + IntToStr(Length(Skipped)) + LineEnding +
        'note=alignment, tree inference, and Newick export use MEGA 12.1 source components.' + LineEnding);
      WriteResponse(Request, '', Skipped);
    except
      on E: Exception do
      begin
        if Length(Skipped) = 0 then
          Skipped := RuntimeSkippedRecordsForError(Request, E.Message);
        WriteResponse(Request, E.Message, Skipped);
        raise;
      end;
    end;
  finally
    Names.Free;
    Seqs.Free;
  end;
end;

begin
  try
    if (ParamCount = 1) and SameText(ParamStr(1), RuntimeProbeArgument) then
    begin
      WriteLn(RuntimeName + ' probe ok');
      Halt(0);
    end;
    if ParamCount <> 1 then
      raise Exception.Create('usage: mega-phgo-runtime <runtime-request.json>');
    RunRequest(ParamStr(1));
  except
    on E: Exception do
    begin
      WriteLn(StdErr, RuntimeName + ': ' + E.Message);
      Halt(1);
    end;
  end;
end.
