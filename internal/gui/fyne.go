//go:build fyne

package gui

import (
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/name-cho/uniland-go/internal/interp/val"
)

func Available() bool { return true }

func RunWindow(spec WindowSpec, call CallFn) (map[string]val.Value, error) {
	a := app.New()
	win := a.NewWindow(spec.Title)

	out := map[string]val.Value{}
	type entryRef struct {
		key string
		w   *widget.Entry
	}
	type checkRef struct {
		key string
		w   *widget.Check
	}
	var entries []entryRef
	var checks []checkRef

	items := make([]fyne.CanvasObject, 0, len(spec.Widgets))
	for _, w := range spec.Widgets {
		switch w.Type {
		case "label":
			items = append(items, widget.NewLabel(w.Text))
		case "entry":
			e := widget.NewEntry()
			e.SetText(w.Text)
			if w.Key != "" {
				entries = append(entries, entryRef{key: w.Key, w: e})
			}
			items = append(items, e)
		case "check":
			c := widget.NewCheck(w.Text, nil)
			c.SetChecked(w.Checked)
			if w.Key != "" {
				checks = append(checks, checkRef{key: w.Key, w: c})
			}
			items = append(items, c)
		case "button":
			onClick := w.OnClick
			b := widget.NewButton(w.Text, func() {
				if call != nil && onClick != nil {
					_, _ = call(onClick, nil)
				}
			})
			items = append(items, b)
		case "separator":
			items = append(items, widget.NewSeparator())
		}
	}

	win.SetContent(container.NewVBox(items...))
	if spec.Width > 0 || spec.Height > 0 {
		win.Resize(fyne.NewSize(float32(spec.Width), float32(spec.Height)))
	}

	done := make(chan struct{})
	win.SetOnClosed(func() {
		for _, e := range entries {
			out[e.key] = val.Str(e.w.Text)
		}
		for _, c := range checks {
			out[c.key] = val.Bool(c.w.Checked)
		}
		close(done)
	})

	win.ShowAndRun()
	<-done
	return out, nil
}

func Alert(title, text string) error {
	a := app.New()
	win := a.NewWindow(title)
	done := make(chan struct{})
	var once sync.Once
	closeOnce := func() { once.Do(func() { close(done) }) }
	dlg := dialog.NewInformation(title, text, win)
	dlg.SetOnClosed(closeOnce)
	dlg.Show()
	<-done
	return nil
}

func Ask(title, text, initial string) (val.Value, error) {
	a := app.New()
	win := a.NewWindow(title)
	entry := widget.NewEntry()
	entry.SetText(initial)

	res := make(chan val.Value, 1)
	var once sync.Once
	send := func(v val.Value) { once.Do(func() { res <- v }) }

	dlg := dialog.NewCustomConfirm(title, "OK", "Отмена",
		container.NewVBox(widget.NewLabel(text), entry),
		func(ok bool) {
			if ok {
				send(val.Str(entry.Text))
			} else {
				send(val.Nul)
			}
		}, win)
	dlg.SetOnClosed(func() { send(val.Nul) })
	dlg.Show()
	return <-res, nil
}

func Confirm(title, text string) (bool, error) {
	a := app.New()
	win := a.NewWindow(title)
	res := make(chan bool, 1)
	var once sync.Once
	send := func(v bool) { once.Do(func() { res <- v }) }
	dlg := dialog.NewConfirm(title, text, func(ok bool) { send(ok) }, win)
	dlg.SetOnClosed(func() { send(false) })
	dlg.Show()
	return <-res, nil
}
