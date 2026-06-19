unit ExcelWrite;

{$IFDEF FPC}
  {$MODE Delphi}
{$ENDIF}

interface

uses
  Classes, SysUtils, Variants, Graphics, Types, MegaConsts;

const
  XLROTATIONNONE = 0;
  XLROTATIONTEXTVERTICALCHARHORIZONTAL = 255;
  XLROTATIONVERTICALUP = 90;
  XLROTATIONVERTICALDOWN = 180;
  MAX_COLS = 1024;

type
  TAlign = (aTop, aCenter, aBottom, aLeft, aRight, aNone);
  TCellBorderIndex = (xlBorderLeft, xlBorderRight, xlBorderTop, xlBorderBottom, xlBorderAll);

  TExcelWrite = class
  public
    Font: TFont;
    CurStr: AnsiString;
    ColumnSize: Array[1..1025] of Integer;
    ShowPleaseWait: Boolean;
    constructor Create(Sender: TObject = nil; FirstSheetName: String = 'Tab1');
    destructor Destroy; override;
    function ChangeFileExtForOutputFileType(filename: String; aType: TOutputFileType): String;
    procedure BoldCells(aRect: TRect);
    procedure ItalicizeCells(aRect: TRect);
    procedure BoldItalicizeCells(aRect: TRect);
    procedure SetFontSize(Worksheet, Column, Row, FontSize: integer);
    procedure SetFontColor(Worksheet, Column, Row: Integer; FontColor: TColor);
    procedure WrapRow(WorkSheet, RowToWrap: Integer; NumCols: Integer);
    procedure SetColumnWidth(Worksheet, Col, NumChars: Integer);
    procedure SetRowHeight(Worksheet, Row, HeightInChars: Integer);
    function Add(toadd: Variant; highlight: TColor = clWhite; fontcolor: TColor = clWhite; rotate: integer = xlRotationNone): Boolean;
    function AddBlankCell(highlight: TColor = clWhite): Boolean;
    function AddBlankCells(NumBlankCells: Integer; highlight: TColor = clWhite): Boolean;
    function AddP(toadd: Variant; Precision: Integer; highlight: TColor = clWhite; fontcolor: TColor = clWhite): Boolean;
    procedure Empty;
    procedure AlignCells(aRect: TRect; HAlign: TAlign = aNone; VAlign: TAlign = aNone; WorkSheet: Integer = 0);
    procedure WriteKeypair(first: Variant; second: Variant; Worksheet: Integer = 0; Start: AnsiChar = 'A');
    procedure WriteLine(Worksheet: Integer = 0; Start: AnsiChar = 'A'; aFormat: String = ''; MergeRow: Boolean = False); overload;
    procedure WriteLine(Line: Variant; Worksheet: Integer = 0; Start: Char = 'A'; Format: String = ''; Highlight: TColor = clWhite); overload;
    procedure ApplyFormat(Left, Top, Right, Bottom: Integer; numFormat: String; worksheet: Integer = 0);
    function AddWorksheet(SheetName: String = 'WorkSheet'): Integer;
    procedure RemoveWorksheet(index: Integer);
    procedure AddCaptionAsWorksheet(captionStrings: TStringList; aName: String = 'Caption'); overload;
    procedure AddCaptionAsWorksheet(captionStrings: TStringList; sheet: Integer); overload;
    procedure NewLine(worksheet: Integer = 0);
    procedure SetIsExcelOutput(XLS: Boolean);
    procedure HadError;
    procedure MergeCells(aRect: TRect; HAlign: TAlign = aNone; VAlign: TAlign = aNone);
    procedure ColorCells(aRect: TRect; Color: TColor; BorderType: TCellBorderIndex = xlBorderAll; Sheet: Integer = 0);
    function LastCellWriteXY(worksheet: integer = 0): TRect;
    procedure ColWidth(Range: TRect; Width, worksheet: Integer);
    procedure AutoSizeColumns;
    function SaveFile(SaveTo: String; OutFormat: TOutputFileType; IsInGuiThread: Boolean = True): Boolean;
    function SaveFileNoAutoFit(SaveTo: String): Boolean;
    function GetCsvText: String;
    function GetCsvList: TStringList;
    function GetJsonText: String;
    function GetAsTabDelimitedText: String;
    function GetAsTabDelimitedList: TStringList;
  private
    FVisible: Boolean;
    FIsXLS: Boolean;
    FAutoFitCols: Boolean;
    function GetVisible: Boolean;
    procedure SetVisible(AValue: Boolean);
    function GetIsXLS: Boolean;
    function GetAutoFitCols: Boolean;
    procedure SetAutoFitCols(AValue: Boolean);
    function GetOutputLineLvl(Index: Integer): Integer;
    function GetOutputPos: Integer;
  public
    property Visible: Boolean read GetVisible write SetVisible;
    property IsXLS: Boolean read GetIsXLS write SetIsExcelOutput;
    property AutoFitCols: Boolean read GetAutoFitCols write SetAutoFitCols;
    property OutputLineLvl[Index: Integer]: Integer read GetOutputLineLvl;
    property OutputPos: Integer read GetOutputPos;
  end;

  TExcelWriteDoneCallBack = procedure(AThread: TThread; Status: String) of object;
  TExcelWriteThread = class(TThread)
  public
    Writer: TExcelWrite;
    FileName: String;
    CallBack: TExcelWriteDoneCallBack;
    OnProgress: TNotifyEvent;
    constructor Create(CreateSuspended: Boolean);
    procedure Execute; override;
  end;

function HasExcel(fileExt: String = '.xls'): Boolean;
function GetSaveLocation(ExportType: TExportType = EXnone): String; overload;
function GetSaveLocation(FileFormat: TOutputFileType): String; overload;
function FileExtForExportType(ExportType: TExportType): String;
function OutputFileTypeForFileExt(FileExt: String): TOutputFileType;
procedure RunAProgram(const theProgram: String);

implementation

constructor TExcelWrite.Create(Sender: TObject; FirstSheetName: String);
begin
  inherited Create;
  Font := TFont.Create;
end;

destructor TExcelWrite.Destroy;
begin
  Font.Free;
  inherited Destroy;
end;

function TExcelWrite.ChangeFileExtForOutputFileType(filename: String; aType: TOutputFileType): String;
begin
  Result := filename;
end;

procedure TExcelWrite.BoldCells(aRect: TRect);
begin
end;

procedure TExcelWrite.ItalicizeCells(aRect: TRect);
begin
end;

procedure TExcelWrite.BoldItalicizeCells(aRect: TRect);
begin
end;

procedure TExcelWrite.SetFontSize(Worksheet, Column, Row, FontSize: integer);
begin
end;

procedure TExcelWrite.SetFontColor(Worksheet, Column, Row: Integer; FontColor: TColor);
begin
end;

procedure TExcelWrite.WrapRow(WorkSheet, RowToWrap: Integer; NumCols: Integer);
begin
end;

procedure TExcelWrite.SetColumnWidth(Worksheet, Col, NumChars: Integer);
begin
end;

procedure TExcelWrite.SetRowHeight(Worksheet, Row, HeightInChars: Integer);
begin
end;

function TExcelWrite.Add(toadd: Variant; highlight: TColor; fontcolor: TColor; rotate: integer): Boolean;
begin
  Result := True;
end;

function TExcelWrite.AddBlankCell(highlight: TColor): Boolean;
begin
  Result := True;
end;

function TExcelWrite.AddBlankCells(NumBlankCells: Integer; highlight: TColor): Boolean;
begin
  Result := True;
end;

function TExcelWrite.AddP(toadd: Variant; Precision: Integer; highlight: TColor; fontcolor: TColor): Boolean;
begin
  Result := Add(toadd, highlight, fontcolor);
end;

procedure TExcelWrite.Empty;
begin
end;

procedure TExcelWrite.AlignCells(aRect: TRect; HAlign: TAlign; VAlign: TAlign; WorkSheet: Integer);
begin
end;

procedure TExcelWrite.WriteKeypair(first: Variant; second: Variant; Worksheet: Integer; Start: AnsiChar);
begin
end;

procedure TExcelWrite.WriteLine(Worksheet: Integer; Start: AnsiChar; aFormat: String; MergeRow: Boolean);
begin
end;

procedure TExcelWrite.WriteLine(Line: Variant; Worksheet: Integer; Start: Char; Format: String; Highlight: TColor);
begin
end;

procedure TExcelWrite.ApplyFormat(Left, Top, Right, Bottom: Integer; numFormat: String; worksheet: Integer);
begin
end;

function TExcelWrite.AddWorksheet(SheetName: String): Integer;
begin
  Result := 0;
end;

procedure TExcelWrite.RemoveWorksheet(index: Integer);
begin
end;

procedure TExcelWrite.AddCaptionAsWorksheet(captionStrings: TStringList; aName: String);
begin
end;

procedure TExcelWrite.AddCaptionAsWorksheet(captionStrings: TStringList; sheet: Integer);
begin
end;

procedure TExcelWrite.NewLine(worksheet: Integer);
begin
end;

procedure TExcelWrite.SetIsExcelOutput(XLS: Boolean);
begin
  FIsXLS := XLS;
end;

procedure TExcelWrite.HadError;
begin
end;

procedure TExcelWrite.MergeCells(aRect: TRect; HAlign: TAlign; VAlign: TAlign);
begin
end;

procedure TExcelWrite.ColorCells(aRect: TRect; Color: TColor; BorderType: TCellBorderIndex; Sheet: Integer);
begin
end;

function TExcelWrite.LastCellWriteXY(worksheet: integer): TRect;
begin
  Result := Rect(0, 0, 0, 0);
end;

procedure TExcelWrite.ColWidth(Range: TRect; Width, worksheet: Integer);
begin
end;

procedure TExcelWrite.AutoSizeColumns;
begin
end;

function TExcelWrite.SaveFile(SaveTo: String; OutFormat: TOutputFileType; IsInGuiThread: Boolean): Boolean;
begin
  Result := False;
end;

function TExcelWrite.SaveFileNoAutoFit(SaveTo: String): Boolean;
begin
  Result := False;
end;

function TExcelWrite.GetCsvText: String;
begin
  Result := '';
end;

function TExcelWrite.GetCsvList: TStringList;
begin
  Result := TStringList.Create;
end;

function TExcelWrite.GetJsonText: String;
begin
  Result := '';
end;

function TExcelWrite.GetAsTabDelimitedText: String;
begin
  Result := '';
end;

function TExcelWrite.GetAsTabDelimitedList: TStringList;
begin
  Result := TStringList.Create;
end;

function TExcelWrite.GetVisible: Boolean;
begin
  Result := FVisible;
end;

procedure TExcelWrite.SetVisible(AValue: Boolean);
begin
  FVisible := AValue;
end;

function TExcelWrite.GetIsXLS: Boolean;
begin
  Result := FIsXLS;
end;

function TExcelWrite.GetAutoFitCols: Boolean;
begin
  Result := FAutoFitCols;
end;

procedure TExcelWrite.SetAutoFitCols(AValue: Boolean);
begin
  FAutoFitCols := AValue;
end;

function TExcelWrite.GetOutputLineLvl(Index: Integer): Integer;
begin
  Result := 1;
end;

function TExcelWrite.GetOutputPos: Integer;
begin
  Result := 1;
end;

constructor TExcelWriteThread.Create(CreateSuspended: Boolean);
begin
  inherited Create(CreateSuspended);
end;

procedure TExcelWriteThread.Execute;
begin
  if Assigned(CallBack) then
    CallBack(Self, '');
end;

function HasExcel(fileExt: String): Boolean;
begin
  Result := False;
end;

function GetSaveLocation(ExportType: TExportType): String;
begin
  Result := '';
end;

function GetSaveLocation(FileFormat: TOutputFileType): String;
begin
  Result := '';
end;

function FileExtForExportType(ExportType: TExportType): String;
begin
  Result := '';
end;

function OutputFileTypeForFileExt(FileExt: String): TOutputFileType;
begin
  Result := ExportInvalid;
end;

procedure RunAProgram(const theProgram: String);
begin
end;

end.
