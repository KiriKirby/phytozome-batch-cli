unit MD_MegaMain;

{$IFDEF FPC}
  {$MODE Delphi}
{$ENDIF}

interface

uses
  Classes, SysUtils, KeywordConsts, MegaConsts, MProcessPack, MAnalysisSummary;

type
  TMegaAction = (
    maFindAlignment,
    maFindTrees,
    maFindTipDates,
    maProcessCommands,
    maConcatenateAlignments,
    maCloneFinder,
    maDeveloper,
    maCompareBootstrapTrees,
    maFindSitePatterns,
    maCompareNodeHeights,
    maNewickToTabular,
    maCompareTopologies,
    maComputeRfDistance,
    maPruneTimetree,
    maDrPhylo,
    maDomainsToFasta,
    maSegmentsToFasta,
    maGenerateTopHapAlignment
  );

  { TD_MegaMain }

  TD_MegaMain = class
  private
    FAnalysisSummary: TAnalysisSummary;
    FCodeTable: AnsiString;
    FCodeTableName: AnsiString;
    FDataDescription: AnsiString;
    FDataFileName: AnsiString;
    FDataTitle: AnsiString;
    FMegaAction: TMegaAction;
    FProcessPack: TProcessPack;
    FRunSilent: Boolean;
  public
    AddBclStdDevToNewick: Boolean;
    BenchMarksFile: String;
    DataInfoGridNV: TStringList;
    DoAllFiles: Boolean;
    ExportSearchTrees: Boolean;
    FDataType: TSnTokenCode;
    FFileShouldContainCoding: Boolean;
    GroupsFilename: String;
    OutputFileName: AnsiString;
    OutputFormat: TOutputFileType;
    AnalysisPreferencesFileName: AnsiString;
    CommandLineTreeFileName: AnsiString;
    CommandLineControlFileName: AnsiString;
    PhyloQFileName: AnsiString;
    IsFileIterator: Boolean;
    IsRunningInTestHarness: Boolean;
    MaxNumResults: Integer;
    PartitionFrequencyCutoff: Double;

    constructor Create;
    destructor Destroy; override;

    procedure DataInfoGridClearAll;
    procedure DataInfoGridSetInfo(InfoType, Info: AnsiString);
    procedure DataInfoGridAddIfDoesntExist(InfoType: AnsiString);
    function DataInfoGridHasInfo(InfoType: AnsiString): Boolean;
    procedure DataInfoGridSetInfoObject(InfoType: AnsiString; Info: Pointer);
    procedure DataInfoGridRemoveInfo(InfoType: AnsiString);
    function DataInfoGridGetInfo(InfoType: AnsiString): AnsiString;
    function DataInfoGridGetInfoObject(InfoType: AnsiString): Pointer;
    procedure AdaptiveModelTestDone(aThread: TObject);
    procedure ConcatenateAlignmentsDone(aThread: TObject);
    procedure DistCommandThreadDone(aThread: TObject);
    procedure DistTreeThreadDone(aThread: TObject);
    procedure EslThreadDone(aThread: TObject);
    procedure ModelTestDone(aThread: TObject);
    procedure SiteCoverageThreadDone(aThread: TObject);
    procedure PrintHelp;
    function UserHasProvidedOutgroupFile: Boolean;

    property AnalysisSummary: TAnalysisSummary read FAnalysisSummary write FAnalysisSummary;
    property CodeTable: AnsiString read FCodeTable write FCodeTable;
    property CodeTableName: AnsiString read FCodeTableName write FCodeTableName;
    property DataFileName: AnsiString read FDataFileName write FDataFileName;
    property DataTitle: AnsiString read FDataTitle write FDataTitle;
    property DataDescription: AnsiString read FDataDescription write FDataDescription;
    property MegaAction: TMegaAction read FMegaAction;
    property RunSilent: Boolean read FRunSilent write FRunSilent;
    property ProcessPack: TProcessPack read FProcessPack;
  end;

var
  D_MegaMain: TD_MegaMain;
  ExecutionStartTime: TDateTime;
  LastDeveloperDataDumpTime: TDateTime;
  DeveloperDataDumpInterval: Integer = -1;
  UseFormattedConsoleOutput: Boolean = False;

implementation

function DataInfoObjectKey(const InfoType: AnsiString): String;
begin
  Result := String(InfoType) + #1 + 'object';
end;

constructor TD_MegaMain.Create;
begin
  inherited Create;
  DataInfoGridNV := TStringList.Create;
  DataInfoGridNV.NameValueSeparator := '=';
  FAnalysisSummary := TAnalysisSummary.Create;
  FProcessPack := TProcessPack.Create;
  AddBclStdDevToNewick := False;
  BenchMarksFile := '';
  DoAllFiles := False;
  ExportSearchTrees := False;
  FDataType := snNoToken;
  FFileShouldContainCoding := False;
  FMegaAction := maProcessCommands;
  GroupsFilename := '';
  FRunSilent := True;
  OutputFormat := ExportText;
  MaxNumResults := DefaultMaxResults;
  PartitionFrequencyCutoff := 0.5;
  ExecutionStartTime := Now;
end;

destructor TD_MegaMain.Destroy;
begin
  FProcessPack.Free;
  FAnalysisSummary.Free;
  DataInfoGridNV.Free;
  inherited Destroy;
end;

procedure TD_MegaMain.DataInfoGridClearAll;
begin
  DataInfoGridNV.Clear;
end;

procedure TD_MegaMain.DataInfoGridSetInfo(InfoType, Info: AnsiString);
begin
  DataInfoGridNV.Values[String(InfoType)] := String(Info);
end;

procedure TD_MegaMain.DataInfoGridAddIfDoesntExist(InfoType: AnsiString);
begin
  if not DataInfoGridHasInfo(InfoType) then
    DataInfoGridSetInfo(InfoType, '');
end;

function TD_MegaMain.DataInfoGridHasInfo(InfoType: AnsiString): Boolean;
begin
  Result := DataInfoGridNV.IndexOfName(String(InfoType)) >= 0;
end;

procedure TD_MegaMain.DataInfoGridSetInfoObject(InfoType: AnsiString; Info: Pointer);
begin
  DataInfoGridNV.Values[DataInfoObjectKey(InfoType)] := IntToStr(PtrUInt(Info));
end;

procedure TD_MegaMain.DataInfoGridRemoveInfo(InfoType: AnsiString);
var
  Index: Integer;
begin
  Index := DataInfoGridNV.IndexOfName(String(InfoType));
  if Index >= 0 then
    DataInfoGridNV.Delete(Index);
  Index := DataInfoGridNV.IndexOfName(DataInfoObjectKey(InfoType));
  if Index >= 0 then
    DataInfoGridNV.Delete(Index);
end;

function TD_MegaMain.DataInfoGridGetInfo(InfoType: AnsiString): AnsiString;
begin
  Result := AnsiString(DataInfoGridNV.Values[String(InfoType)]);
end;

function TD_MegaMain.DataInfoGridGetInfoObject(InfoType: AnsiString): Pointer;
var
  Raw: String;
  Value: QWord;
begin
  Result := nil;
  Raw := DataInfoGridNV.Values[DataInfoObjectKey(InfoType)];
  if Raw = '' then
    Exit;
  if TryStrToQWord(Raw, Value) then
    Result := Pointer(PtrUInt(Value));
end;

procedure TD_MegaMain.PrintHelp;
begin
end;

procedure TD_MegaMain.AdaptiveModelTestDone(aThread: TObject);
begin
end;

procedure TD_MegaMain.ConcatenateAlignmentsDone(aThread: TObject);
begin
end;

procedure TD_MegaMain.DistTreeThreadDone(aThread: TObject);
begin
end;

procedure TD_MegaMain.DistCommandThreadDone(aThread: TObject);
begin
end;

procedure TD_MegaMain.EslThreadDone(aThread: TObject);
begin
end;

procedure TD_MegaMain.ModelTestDone(aThread: TObject);
begin
end;

procedure TD_MegaMain.SiteCoverageThreadDone(aThread: TObject);
begin
end;

function TD_MegaMain.UserHasProvidedOutgroupFile: Boolean;
var
  Groups: TStringList;
  I: Integer;
begin
  Result := False;
  if (Trim(GroupsFilename) = '') or (not FileExists(GroupsFilename)) then
    Exit;

  Groups := TStringList.Create;
  try
    Groups.LoadFromFile(GroupsFilename);
    for I := 0 to Groups.Count - 1 do
      if SameText(Groups.ValueFromIndex[I], 'outgroup') then
        Exit(True);
  finally
    Groups.Free;
  end;
end;

initialization
  D_MegaMain := TD_MegaMain.Create;

finalization
  FreeAndNil(D_MegaMain);

end.
