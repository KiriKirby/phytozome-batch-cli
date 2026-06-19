unit mmega_std_out;

{$mode objfpc}{$H+}

interface

uses
  Classes, SysUtils;

procedure DoTextOut(x, y: Word; const s: String; aColor: Word = 15); overload;
procedure DoTextOut(x, y: Word; const key: String; const value: String); overload;
procedure ClearMegaStdOut;
function HasColorCapability: Boolean;
procedure GetColorForOutputType(const aType: String; var nameColor: Word; var valueColor: Word);
function NextProgressLine: Word;

const
  STATUS_MARGIN = 4;
  CURRENT_PROGRESS = 'Current Progress';

var
  MegaStdOut: TStringList;
  MegaProgressStrings: TStringList;
  ProgressNameColumnWidth: Integer;
  ProgressScreenWidth: Integer;
  ProgressScreenHeight: Integer;
  ProgressCurrentPosition: Integer;
  StaticProgressPosition: Integer;
  NumLinesWritten: Integer;
  LastKeyWritten: String;
  LastValueWritten: String;

implementation

procedure DoTextOut(x, y: Word; const s: String; aColor: Word = 15);
begin
  LastValueWritten := s;
  Inc(NumLinesWritten);
  if Assigned(MegaStdOut) then
    MegaStdOut.Add(s);
end;

procedure DoTextOut(x, y: Word; const key: String; const value: String);
begin
  LastKeyWritten := key;
  LastValueWritten := value;
  Inc(NumLinesWritten);
  if Assigned(MegaStdOut) then
    MegaStdOut.Add(key + ': ' + value);
end;

procedure ClearMegaStdOut;
begin
  NumLinesWritten := 0;
  LastKeyWritten := '';
  LastValueWritten := '';
  if Assigned(MegaStdOut) then
    MegaStdOut.Clear;
end;

function HasColorCapability: Boolean;
begin
  Result := False;
end;

procedure GetColorForOutputType(const aType: String; var nameColor: Word; var valueColor: Word);
begin
  nameColor := 15;
  valueColor := 15;
end;

function NextProgressLine: Word;
begin
  Result := NumLinesWritten + 1;
end;

initialization
  MegaStdOut := TStringList.Create;
  MegaProgressStrings := TStringList.Create;
  ProgressNameColumnWidth := 20;
  ProgressScreenWidth := 80;
  ProgressScreenHeight := 25;

finalization
  FreeAndNil(MegaStdOut);
  FreeAndNil(MegaProgressStrings);

end.
