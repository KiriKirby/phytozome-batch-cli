package tui

import (
	"reflect"
	"strings"
	"testing"
	"time"
	"unsafe"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func writableApplicationField(app *tview.Application, name string) reflect.Value {
	if app == nil || strings.TrimSpace(name) == "" {
		return reflect.Value{}
	}
	value := reflect.ValueOf(app)
	if !value.IsValid() || value.Kind() != reflect.Pointer || value.IsNil() {
		return reflect.Value{}
	}
	field := value.Elem().FieldByName(name)
	if !field.IsValid() || !field.CanAddr() {
		return reflect.Value{}
	}
	return reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem()
}

func applicationRoot(app *tview.Application) tview.Primitive {
	field := writableApplicationField(app, "root")
	if !field.IsValid() || field.IsNil() {
		return nil
	}
	root, _ := field.Interface().(tview.Primitive)
	return root
}

func TestSetPageRootResetsApplicationMouseState(t *testing.T) {
	app := newApp()
	writableApplicationField(app, "mouseCapturingPrimitive").Set(reflect.ValueOf(tview.NewBox()))
	writableApplicationField(app, "lastMouseButtons").Set(reflect.ValueOf(tcell.ButtonPrimary))
	writableApplicationField(app, "mouseDownX").SetInt(33)
	writableApplicationField(app, "mouseDownY").SetInt(17)
	writableApplicationField(app, "lastMouseX").SetInt(19)
	writableApplicationField(app, "lastMouseY").SetInt(21)
	writableApplicationField(app, "lastMouseClick").Set(reflect.ValueOf(time.Now()))

	setPageRoot(app, tview.NewBox())

	for _, name := range []string{"mouseCapturingPrimitive", "lastMouseButtons", "mouseDownX", "mouseDownY", "lastMouseX", "lastMouseY", "lastMouseClick"} {
		field := writableApplicationField(app, name)
		if !field.IsValid() {
			t.Fatalf("missing application field %q", name)
		}
		if !field.IsZero() {
			t.Fatalf("application field %q not reset: %#v", name, field.Interface())
		}
	}
}

func TestFamilyBlastRenameModalShowsPasteButton(t *testing.T) {
	app := newApp()
	var result FamilyBlastResult
	buildFamilyBlastCustomizeModal(FamilyBlastCustomizePage{
		Title: "Customize Family BLAST groups",
		Groups: []FamilyBlastCustomGroup{
			{Name: "PAL", Labels: []string{"PAL1", "PAL2"}},
		},
		Ungrouped: []string{"X1"},
		AllowBack: true,
	}, app, &result)

	capture := app.GetInputCapture()
	if capture == nil {
		t.Fatal("expected input capture to be installed")
	}
	capture(tcell.NewEventKey(tcell.KeyF2, 0, 0))

	input, ok := app.GetFocus().(*tview.InputField)
	if !ok {
		t.Fatalf("focus after F2 = %T, want *tview.InputField", app.GetFocus())
	}
	if got := input.GetFieldWidth(); got != renameDialogFieldWidth {
		t.Fatalf("rename input width = %d, want %d", got, renameDialogFieldWidth)
	}

	root := applicationRoot(app)
	if root == nil {
		t.Fatal("expected current modal root")
	}
	screen := tcell.NewSimulationScreen("")
	if err := screen.Init(); err != nil {
		t.Fatalf("screen init failed: %v", err)
	}
	screen.SetSize(160, 40)
	root.SetRect(0, 0, 160, 40)
	root.Draw(screen)
	if !screenContains(screen, 160, 40, "Paste (Ctrl+V)") {
		t.Fatal("rename modal should render a Paste (Ctrl+V) button")
	}
}
