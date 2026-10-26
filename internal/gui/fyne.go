//go:build cgo && (!nogui || fyne)

package gui

import (
	"bytes"
	"errors"
	"fmt"
	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/webp"
	"image"
	"image/gif"
	_ "image/jpeg"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/ncruces/zenity"
	xdraw "golang.org/x/image/draw"

	"github.com/name-cho/uniland-go/internal/interp/val"
)

type handle struct {
	kind  string
	win   fyne.Window
	box   *fyne.Container
	obj   fyne.CanvasObject
	close []func()
}

var (
	appOnce     sync.Once
	appInst     fyne.App
	loopRunning bool
	loopDone    bool
)

func application() fyne.App {
	appOnce.Do(func() { appInst = app.New() })
	return appInst
}

func loopErr(name string) error {
	if loopDone {
		return fmt.Errorf("%s(): цикл событий GUI уже завершён, повторный запуск не поддерживается", name)
	}
	return nil
}

func handleOf(v val.Value) (*handle, bool) {
	o, ok := v.(*val.Opaque)
	if !ok {
		return nil, false
	}
	h, ok := o.V.(*handle)
	return h, ok
}

func windowOf(v val.Value, name string) (*handle, error) {
	if err := loopErr(name); err != nil {
		return nil, err
	}
	h, ok := handleOf(v)
	if !ok || h.kind != "window" || h.win == nil {
		return nil, fmt.Errorf("%s() needs a parent window", name)
	}
	return h, nil
}

func Window(title string, width, height int) (val.Value, error) {
	if err := loopErr("gui_window"); err != nil {
		return nil, err
	}
	w := application().NewWindow(title)
	box := container.NewVBox()
	w.SetContent(box)
	h := &handle{kind: "window", win: w, box: box}
	w.SetOnClosed(func() {
		for _, fn := range h.close {
			fn()
		}
	})
	if width > 0 || height > 0 {
		w.Resize(fyne.NewSize(float32(width), float32(height)))
	}
	return val.NewOpaque(h), nil
}

func Label(win val.Value, text string) (val.Value, error) {
	h, err := windowOf(win, "gui_label")
	if err != nil {
		return nil, err
	}
	l := widget.NewLabel(text)
	h.box.Add(l)
	return val.NewOpaque(&handle{kind: "label", win: h.win, obj: l}), nil
}

func Button(win val.Value, text string, fn val.Value, call CallFn) (val.Value, error) {
	h, err := windowOf(win, "gui_button")
	if err != nil {
		return nil, err
	}
	b := widget.NewButton(text, func() {
		if fn != nil && call != nil {
			_, _ = call(fn, nil)
		}
	})
	h.box.Add(b)
	return val.NewOpaque(&handle{kind: "button", win: h.win, obj: b}), nil
}

func Input(win val.Value, initial string) (val.Value, error) {
	h, err := windowOf(win, "gui_input")
	if err != nil {
		return nil, err
	}
	e := widget.NewEntry()
	e.SetText(initial)
	h.box.Add(e)
	return val.NewOpaque(&handle{kind: "entry", win: h.win, obj: e}), nil
}

func Get(w val.Value) (val.Value, error) {
	if err := loopErr("gui_get"); err != nil {
		return nil, err
	}
	h, ok := handleOf(w)
	if !ok {
		return nil, errors.New("gui_get() needs a widget")
	}
	switch o := h.obj.(type) {
	case *widget.Entry:
		return val.Str(o.Text), nil
	case *widget.Label:
		return val.Str(o.Text), nil
	case *widget.Button:
		return val.Str(o.Text), nil
	}
	return val.Str(""), nil
}

func Set(w val.Value, text string) error {
	if err := loopErr("gui_set"); err != nil {
		return err
	}
	h, ok := handleOf(w)
	if !ok {
		return errors.New("gui_set() needs a widget")
	}
	switch o := h.obj.(type) {
	case *widget.Entry:
		o.SetText(text)
	case *widget.Label:
		o.SetText(text)
	case *widget.Button:
		o.SetText(text)
	default:
		return errors.New("gui_set(): widget does not support text")
	}
	return nil
}

func Run(win val.Value) error {
	h, ok := handleOf(win)
	if !ok || h.kind != "window" || h.win == nil {
		return errors.New("gui_run() needs a window")
	}
	if err := loopErr("gui_run"); err != nil {
		return err
	}
	if loopRunning {
		return errors.New("gui_run(): цикл событий GUI уже запущен")
	}
	loopRunning = true
	h.win.ShowAndRun()
	loopRunning = false
	loopDone = true
	return nil
}

func Close(win val.Value) error {
	if err := loopErr("gui_close"); err != nil {
		return err
	}
	h, ok := handleOf(win)
	if !ok || h.kind != "window" || h.win == nil {
		return errors.New("gui_close() needs a window")
	}
	h.win.Close()
	return nil
}

func OnClose(win val.Value, fn val.Value, call CallFn) error {
	if err := loopErr("gui_on_close"); err != nil {
		return err
	}
	h, ok := handleOf(win)
	if !ok || h.kind != "window" || h.win == nil || fn == nil {
		return errors.New("gui_on_close() needs a window and a function")
	}
	h.close = append(h.close, func() {
		if call != nil {
			_, _ = call(fn, nil)
		}
	})
	return nil
}

func loadImage(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("Не удалось загрузить изображение '%s': %v", path, err)
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("Не удалось загрузить изображение '%s': %v", path, err)
	}
	return img, nil
}

func resizeImage(src image.Image, width, height int) image.Image {
	if width <= 0 || height <= 0 {
		return src
	}
	dst := image.NewRGBA(image.Rect(0, 0, width, height))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), xdraw.Over, nil)
	return dst
}

func Image(win val.Value, path string, width, height int) (val.Value, error) {
	h, err := windowOf(win, "gui_image")
	if err != nil {
		return nil, err
	}
	img, err := loadImage(path)
	if err != nil {
		return nil, err
	}
	ci := canvas.NewImageFromImage(resizeImage(img, width, height))
	ci.FillMode = canvas.ImageFillOriginal
	h.box.Add(ci)
	return val.NewOpaque(&handle{kind: "image", win: h.win, obj: ci}), nil
}

func ImageButton(win val.Value, path string, fn val.Value, width, height int, call CallFn) (val.Value, error) {
	h, err := windowOf(win, "gui_image_button")
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("Не удалось загрузить изображение '%s': %v", path, err)
	}
	if width > 0 && height > 0 {
		img, err := loadImage(path)
		if err != nil {
			return nil, err
		}
		var buf bytes.Buffer
		if err := png.Encode(&buf, resizeImage(img, width, height)); err != nil {
			return nil, err
		}
		data = buf.Bytes()
	}
	b := widget.NewButtonWithIcon("", fyne.NewStaticResource(filepath.Base(path), data), func() {
		if fn != nil && call != nil {
			_, _ = call(fn, nil)
		}
	})
	h.box.Add(b)
	return val.NewOpaque(&handle{kind: "image_button", win: h.win, obj: b}), nil
}

func Gif(win val.Value, path string, width, height int) (val.Value, error) {
	h, err := windowOf(win, "gui_gif")
	if err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("Не удалось загрузить GIF '%s': %v", path, err)
	}
	defer f.Close()
	anim, err := gif.DecodeAll(f)
	if err != nil {
		return nil, fmt.Errorf("Не удалось загрузить GIF '%s': %v", path, err)
	}
	if len(anim.Image) == 0 {
		return nil, fmt.Errorf("В файле нет кадров GIF: %s", path)
	}
	frames := make([]image.Image, len(anim.Image))
	for i, fr := range anim.Image {
		frames[i] = resizeImage(fr, width, height)
	}
	ci := canvas.NewImageFromImage(frames[0])
	ci.FillMode = canvas.ImageFillOriginal
	h.box.Add(ci)
	if len(frames) > 1 {
		stop := make(chan struct{})
		var once sync.Once
		h.close = append(h.close, func() { once.Do(func() { close(stop) }) })
		delayAt := func(idx int) time.Duration {
			d := 10
			if idx < len(anim.Delay) && anim.Delay[idx] > 0 {
				d = anim.Delay[idx]
			}
			if d < 2 {
				d = 2
			}
			return time.Duration(d) * 10 * time.Millisecond
		}
		go func() {
			frame := 0
			for {
				select {
				case <-stop:
					return
				case <-time.After(delayAt(frame)):
				}
				frame = (frame + 1) % len(frames)
				img := frames[frame]
				fyne.Do(func() {
					ci.Image = img
					ci.Refresh()
				})
			}
		}()
	}
	return val.NewOpaque(&handle{kind: "gif", win: h.win, obj: ci}), nil
}

func Video(win val.Value, path string) (val.Value, error) {
	if _, err := windowOf(win, "gui_video"); err != nil {
		return nil, err
	}
	fmt.Printf("[INFO] видео открывается во внешнем плеере: %s\n", path)
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", path)
	case "darwin":
		cmd = exec.Command("open", path)
	default:
		cmd = exec.Command("xdg-open", path)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("Не удалось открыть видео '%s': %v", path, err)
	}
	return val.Nul, nil
}

func PickFile(title string) (val.Value, error) {
	p, err := zenity.SelectFile(zenity.Title(title))
	if errors.Is(err, zenity.ErrCanceled) {
		return val.Nul, nil
	}
	if err != nil {
		return nil, err
	}
	return val.Str(p), nil
}

func PickSave(title string) (val.Value, error) {
	p, err := zenity.SelectFileSave(zenity.Title(title))
	if errors.Is(err, zenity.ErrCanceled) {
		return val.Nul, nil
	}
	if err != nil {
		return nil, err
	}
	return val.Str(p), nil
}

func PickFolder(title string) (val.Value, error) {
	p, err := zenity.SelectFile(zenity.Directory(), zenity.Title(title))
	if errors.Is(err, zenity.ErrCanceled) {
		return val.Nul, nil
	}
	if err != nil {
		return nil, err
	}
	return val.Str(p), nil
}

func Alert(message string) error {
	err := zenity.Info(message, zenity.Title("UniLand"))
	if errors.Is(err, zenity.ErrCanceled) {
		return nil
	}
	return err
}

func Confirm(message string) (bool, error) {
	err := zenity.Question(message, zenity.Title("UniLand"))
	if errors.Is(err, zenity.ErrCanceled) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func Prompt(message, initial string) (val.Value, error) {
	s, err := zenity.Entry(message, zenity.Title("UniLand"), zenity.EntryText(initial))
	if errors.Is(err, zenity.ErrCanceled) {
		return val.Nul, nil
	}
	if err != nil {
		return nil, err
	}
	return val.Str(s), nil
}
