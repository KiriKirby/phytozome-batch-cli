unit mruntimeprogressdlg;

{$IFDEF FPC}
  {$MODE Delphi}
{$ENDIF}

interface

uses
  Classes, mcancellable;

type
  TUpdateRunStatusInfoEvent = procedure (AType, AInfo: String) of object;

  TRuntimeProgress = class(TObject)
  private
    FProgress: Integer;
    FThread: TThread;
    FVisible: Boolean;
    FCaption: String;
    FCancellables: TList;
    FFixedColWidthForStr: String;
    FHasCmdLineOutput: Boolean;
    FIsMarqueeMode: Boolean;
    FNeedsTrayIcon: Boolean;
    FCommandLines: TStringList;
    procedure SetHasCmdLineOutput(Cmd: Boolean);
  public
    FMAI: Pointer;
    AnalysisOptStrList: TStringList;
    DataFileName: String;
    DataTitle: String;
    StdOutLabelCaption: String;
    constructor Create(AOwner: TObject = nil);
    destructor Destroy; override;
    procedure AddRunStatusInfo(AType: String; AInfo: String);
    procedure UpdateRunStatusInfo(AType, AInfo: String);
    procedure AddAnalysisOptions(AType, AInfo: String; showOptions: Boolean = True); overload;
    procedure AddAnalysisOptions(aOptions: TStringList; showOptions: Boolean = True); overload;
    procedure RemoveRunStatusInfo(AType: String); overload;
    procedure RemoveRunStatusInfo(aNames: TStringList); overload;
    procedure RemoveAllRunStatusInfo;
    function ProgressCheckCancel(Progress: Integer): Boolean;
    function ProgressAndStatusCheckCancel(Progress: Integer; AType: String; AInfo: String): Boolean;
    function ProgressAndStatusInfoCheckCancel(Progress: Integer; AInfo: String): Boolean;
    procedure RegisterCancellable(c: TCancellable);
    procedure UnregisterCancellable(c: TCancellable);
    procedure Refresh;
    procedure Show;
    procedure Hide;
    procedure HideAnalysisOptions;
    procedure ShowAnalysisOptions;
    procedure Close;
    procedure ProcessMessages;
    procedure SetFixedColWidthForStr(AString: String);
    procedure SetProgress(progress: Integer);
    procedure UpdatePercentProgress(ProgressIn: Integer);
    procedure WriteAnalysisOptionsToStdOut;
    procedure AddCommandLine(NewLine: String);
    function AddCommandLineCheckCancel(NewLine: String): Boolean;
    procedure DisplayCommandLineTab;
    property Progress: Integer read FProgress write FProgress;
    property Thread: TThread read FThread write FThread;
    property Visible: Boolean read FVisible write FVisible;
    property Caption: String read FCaption write FCaption;
    property HasCmdLineOutput: Boolean write SetHasCmdLineOutput;
    property IsMarqueeMode: Boolean read FIsMarqueeMode write FIsMarqueeMode;
    property NeedsTrayIcon: Boolean read FNeedsTrayIcon write FNeedsTrayIcon;
  end;

procedure ShowRunStatusInfoStatic(AType, AInfo: String);
procedure ShowProgressIncrementStatic;

implementation

constructor TRuntimeProgress.Create(AOwner: TObject);
begin
  inherited Create;
  AnalysisOptStrList := TStringList.Create;
  FCommandLines := TStringList.Create;
  FCancellables := TList.Create;
end;

destructor TRuntimeProgress.Destroy;
begin
  FCancellables.Free;
  FCommandLines.Free;
  AnalysisOptStrList.Free;
  inherited Destroy;
end;

procedure TRuntimeProgress.AddRunStatusInfo(AType: String; AInfo: String);
begin
end;

procedure TRuntimeProgress.UpdateRunStatusInfo(AType, AInfo: String);
begin
end;

procedure TRuntimeProgress.AddAnalysisOptions(AType, AInfo: String; showOptions: Boolean);
begin
  if Assigned(AnalysisOptStrList) then
    AnalysisOptStrList.Values[AType] := AInfo;
end;

procedure TRuntimeProgress.AddAnalysisOptions(aOptions: TStringList; showOptions: Boolean);
begin
  if Assigned(AnalysisOptStrList) and Assigned(aOptions) then
    AnalysisOptStrList.Assign(aOptions);
end;

procedure TRuntimeProgress.RemoveRunStatusInfo(AType: String);
begin
end;

procedure TRuntimeProgress.RemoveRunStatusInfo(aNames: TStringList);
begin
end;

procedure TRuntimeProgress.RemoveAllRunStatusInfo;
begin
end;

function TRuntimeProgress.ProgressCheckCancel(Progress: Integer): Boolean;
begin
  Result := False;
end;

function TRuntimeProgress.ProgressAndStatusCheckCancel(Progress: Integer; AType: String; AInfo: String): Boolean;
begin
  Result := False;
end;

function TRuntimeProgress.ProgressAndStatusInfoCheckCancel(Progress: Integer; AInfo: String): Boolean;
begin
  Result := False;
end;

procedure TRuntimeProgress.RegisterCancellable(c: TCancellable);
begin
  if Assigned(c) and Assigned(FCancellables) and (FCancellables.IndexOf(c) < 0) then
    FCancellables.Add(c);
end;

procedure TRuntimeProgress.UnregisterCancellable(c: TCancellable);
begin
  if Assigned(c) and Assigned(FCancellables) then
    FCancellables.Remove(c);
end;

procedure TRuntimeProgress.Refresh;
begin
end;

procedure TRuntimeProgress.Show;
begin
  FVisible := True;
end;

procedure TRuntimeProgress.Hide;
begin
  FVisible := False;
end;

procedure TRuntimeProgress.HideAnalysisOptions;
begin
end;

procedure TRuntimeProgress.ShowAnalysisOptions;
begin
end;

procedure TRuntimeProgress.Close;
begin
  FVisible := False;
end;

procedure TRuntimeProgress.ProcessMessages;
begin
end;

procedure TRuntimeProgress.SetFixedColWidthForStr(AString: String);
begin
  FFixedColWidthForStr := AString;
end;

procedure TRuntimeProgress.SetProgress(progress: Integer);
begin
  FProgress := progress;
end;

procedure TRuntimeProgress.UpdatePercentProgress(ProgressIn: Integer);
begin
  FProgress := ProgressIn;
end;

procedure TRuntimeProgress.WriteAnalysisOptionsToStdOut;
begin
end;

procedure TRuntimeProgress.AddCommandLine(NewLine: String);
begin
  if Assigned(FCommandLines) then
    FCommandLines.Add(NewLine);
end;

function TRuntimeProgress.AddCommandLineCheckCancel(NewLine: String): Boolean;
begin
  AddCommandLine(NewLine);
  Result := False;
end;

procedure TRuntimeProgress.DisplayCommandLineTab;
begin
end;

procedure TRuntimeProgress.SetHasCmdLineOutput(Cmd: Boolean);
begin
  FHasCmdLineOutput := Cmd;
end;

procedure ShowRunStatusInfoStatic(AType, AInfo: String);
begin
end;

procedure ShowProgressIncrementStatic;
begin
end;

end.
