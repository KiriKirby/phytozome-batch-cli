unit MegaUtils_NV;

{$IFDEF FPC}
  {$MODE Delphi}
{$ENDIF}

interface

uses
  Classes, SysUtils, SyncObjs, MegaConsts;

type
  TMEGAThread = class(TThread)
  protected
    FAInfo: TObject;
    FStartTime: TDateTime;
    FEndTime: TDateTime;
    FStartMem: Double;
    FEndMem: Double;
    FIsSuccess: Boolean;
    function GetLogText: String;
    procedure Execute; override;
    procedure StartExecute; virtual;
    procedure EndExecute; virtual;
    function AnalysisDescription: String; virtual;
  public
    SkipSummaryUpdate: Boolean;
    MessagesLog: TStringList;
    constructor Create(Suspended: Boolean);
    destructor Destroy; override;
    procedure UpdateAnalysisSummary; virtual;
    property LogText: String read GetLogText;
    property IsSuccess: Boolean read FIsSuccess;
    property Info: TObject read FAInfo write FAInfo;
  end;

function GroupsFileHasOutgroup(groupsFile: String): Boolean;
function DumpIntArrayToFile(a: TArrayOfInteger; filename: String): Boolean;
function DumpPIntArrayToFile(a: PArrayOfInt; n: Integer; filename: String): Boolean;
function DumpPArrayOfNodeData(p: PArrayOfNodeData; n: Integer; filename: String): Boolean;
function DirectoryExistsCreate(Directory: String): Boolean;
function GetPathToDefaultResultsDirectory: String;
function NextAvailableFilenameNV(NewExtension: String): AnsiString;
function GetFileOutputExtension: String;
procedure error_warning_nv(errorwarning: AnsiString; isError: Boolean);
procedure warn_NV(warning: AnsiString);
procedure error_nv(error: AnsiString; E: Exception = nil);
function LastPos(SubStr, S: AnsiString): Integer;
function OutputFileTypeToExportType(OutputFileType: TOutputFileType): TExportType;

var
  RunningThreadCount: Integer = 0;
  NumActiveThreadsCS: TCriticalSection;

implementation

constructor TMEGAThread.Create(Suspended: Boolean);
begin
  inherited Create(Suspended);
  FreeOnTerminate := False;
  MessagesLog := TStringList.Create;
  FIsSuccess := False;
end;

destructor TMEGAThread.Destroy;
begin
  MessagesLog.Free;
  inherited Destroy;
end;

function TMEGAThread.GetLogText: String;
begin
  if Assigned(MessagesLog) then
    Result := MessagesLog.Text
  else
    Result := '';
end;

procedure TMEGAThread.Execute;
begin
  StartExecute;
  try
    FIsSuccess := True;
  finally
    EndExecute;
  end;
end;

procedure TMEGAThread.StartExecute;
begin
  FStartTime := Now;
  Inc(RunningThreadCount);
end;

procedure TMEGAThread.EndExecute;
begin
  FEndTime := Now;
  if RunningThreadCount > 0 then
    Dec(RunningThreadCount);
end;

function TMEGAThread.AnalysisDescription: String;
begin
  Result := '';
end;

procedure TMEGAThread.UpdateAnalysisSummary;
begin
end;

function GroupsFileHasOutgroup(groupsFile: String): Boolean;
begin
  Result := False;
end;

function DumpIntArrayToFile(a: TArrayOfInteger; filename: String): Boolean;
begin
  Result := False;
end;

function DumpPIntArrayToFile(a: PArrayOfInt; n: Integer; filename: String): Boolean;
begin
  Result := False;
end;

function DumpPArrayOfNodeData(p: PArrayOfNodeData; n: Integer; filename: String): Boolean;
begin
  Result := False;
end;

function DirectoryExistsCreate(Directory: String): Boolean;
begin
  Result := DirectoryExists(Directory) or ForceDirectories(Directory);
end;

function GetPathToDefaultResultsDirectory: String;
begin
  Result := GetCurrentDir + PathDelim;
end;

function NextAvailableFilenameNV(NewExtension: String): AnsiString;
begin
  Result := 'mega-phgo-runtime' + NewExtension;
end;

function GetFileOutputExtension: String;
begin
  Result := '.txt';
end;

procedure error_warning_nv(errorwarning: AnsiString; isError: Boolean);
begin
  if isError then
    raise Exception.Create(String(errorwarning));
end;

procedure warn_NV(warning: AnsiString);
begin
end;

procedure error_nv(error: AnsiString; E: Exception);
begin
  if Assigned(E) then
    raise Exception.Create(String(error) + ': ' + E.Message);
  raise Exception.Create(String(error));
end;

function LastPos(SubStr, S: AnsiString): Integer;
var
  I: Integer;
begin
  Result := 0;
  if (SubStr = '') or (S = '') then
    Exit;
  for I := Length(S) - Length(SubStr) + 1 downto 1 do
    if Copy(S, I, Length(SubStr)) = SubStr then
      Exit(I);
end;

function OutputFileTypeToExportType(OutputFileType: TOutputFileType): TExportType;
begin
  Result := EXnone;
end;

initialization
  NumActiveThreadsCS := TCriticalSection.Create;

finalization
  NumActiveThreadsCS.Free;

end.
