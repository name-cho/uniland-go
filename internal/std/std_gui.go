package std

import (
	"fmt"

	"github.com/name-cho/uniland-go/internal/gui"
	"github.com/name-cho/uniland-go/internal/interp/val"
	"github.com/name-cho/uniland-go/internal/pyops"
)

func argAt(args []val.Value, i int) val.Value {
	if i < len(args) {
		return args[i]
	}
	return nil
}

func argText(args []val.Value, i int, def string) string {
	if i >= len(args) {
		return def
	}
	return pyops.PyStr(args[i])
}

func argInt(args []val.Value, i int, def int) (int, error) {
	if i >= len(args) {
		return def, nil
	}
	n, err := pyops.IntOf(args[i])
	if err != nil {
		return 0, err
	}
	if !n.IsInt64() {
		return 0, fmt.Errorf("int too large to convert to int")
	}
	return int(n.Int64()), nil
}

func callbackArg(args []val.Value, i int) val.Value {
	v := argAt(args, i)
	if v == nil || isNullValue(v) {
		return nil
	}
	return v
}

func bGUIWindow(ctx *Ctx) Native {
	return func(args []val.Value) (val.Value, error) {
		title := argText(args, 0, "UniLand")
		width, err := argInt(args, 1, 400)
		if err != nil {
			return nil, err
		}
		height, err := argInt(args, 2, 300)
		if err != nil {
			return nil, err
		}
		return gui.Window(title, width, height)
	}
}

func bGUILabel(ctx *Ctx) Native {
	return func(args []val.Value) (val.Value, error) {
		return gui.Label(argAt(args, 0), argText(args, 1, ""))
	}
}

func bGUIButton(ctx *Ctx) Native {
	return func(args []val.Value) (val.Value, error) {
		return gui.Button(argAt(args, 0), argText(args, 1, "Button"), callbackArg(args, 2), gui.CallFn(ctx.Call))
	}
}

func bGUIInput(ctx *Ctx) Native {
	return func(args []val.Value) (val.Value, error) {
		return gui.Input(argAt(args, 0), argText(args, 1, ""))
	}
}

func bGUIGet(ctx *Ctx) Native {
	return func(args []val.Value) (val.Value, error) {
		return gui.Get(argAt(args, 0))
	}
}

func bGUISet(ctx *Ctx) Native {
	return func(args []val.Value) (val.Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("gui_set() needs a widget and a value")
		}
		if err := gui.Set(args[0], pyops.PyStr(args[1])); err != nil {
			return nil, err
		}
		return val.Nul, nil
	}
}

func bGUIRun(ctx *Ctx) Native {
	return func(args []val.Value) (val.Value, error) {
		if err := gui.Run(argAt(args, 0)); err != nil {
			return nil, err
		}
		return val.Nul, nil
	}
}

func bGUIClose(ctx *Ctx) Native {
	return func(args []val.Value) (val.Value, error) {
		if err := gui.Close(argAt(args, 0)); err != nil {
			return nil, err
		}
		return val.Nul, nil
	}
}

func bGUIOnClose(ctx *Ctx) Native {
	return func(args []val.Value) (val.Value, error) {
		if err := gui.OnClose(argAt(args, 0), callbackArg(args, 1), gui.CallFn(ctx.Call)); err != nil {
			return nil, err
		}
		return val.Nul, nil
	}
}

func bGUIImage(ctx *Ctx) Native {
	return func(args []val.Value) (val.Value, error) {
		width, err := argInt(args, 2, 0)
		if err != nil {
			return nil, err
		}
		height, err := argInt(args, 3, 0)
		if err != nil {
			return nil, err
		}
		return gui.Image(argAt(args, 0), argText(args, 1, ""), width, height)
	}
}

func bGUIImageButton(ctx *Ctx) Native {
	return func(args []val.Value) (val.Value, error) {
		width, err := argInt(args, 3, 0)
		if err != nil {
			return nil, err
		}
		height, err := argInt(args, 4, 0)
		if err != nil {
			return nil, err
		}
		return gui.ImageButton(argAt(args, 0), argText(args, 1, ""), callbackArg(args, 2), width, height, gui.CallFn(ctx.Call))
	}
}

func bGUIGif(ctx *Ctx) Native {
	return func(args []val.Value) (val.Value, error) {
		width, err := argInt(args, 2, 0)
		if err != nil {
			return nil, err
		}
		height, err := argInt(args, 3, 0)
		if err != nil {
			return nil, err
		}
		return gui.Gif(argAt(args, 0), argText(args, 1, ""), width, height)
	}
}

func bGUIVideo(ctx *Ctx) Native {
	return func(args []val.Value) (val.Value, error) {
		return gui.Video(argAt(args, 0), argText(args, 1, ""))
	}
}

func bPickFile(ctx *Ctx) Native {
	return func(args []val.Value) (val.Value, error) {
		return gui.PickFile(argText(args, 0, "Выбери файл"))
	}
}

func bPickSave(ctx *Ctx) Native {
	return func(args []val.Value) (val.Value, error) {
		return gui.PickSave(argText(args, 0, "Сохранить как"))
	}
}

func bPickFolder(ctx *Ctx) Native {
	return func(args []val.Value) (val.Value, error) {
		return gui.PickFolder(argText(args, 0, "Выбери папку"))
	}
}

func bAlert(ctx *Ctx) Native {
	return func(args []val.Value) (val.Value, error) {
		if err := gui.Alert(argText(args, 0, "")); err != nil {
			return nil, err
		}
		return val.Nul, nil
	}
}

func bConfirm(ctx *Ctx) Native {
	return func(args []val.Value) (val.Value, error) {
		ok, err := gui.Confirm(argText(args, 0, ""))
		if err != nil {
			return nil, err
		}
		return val.Bool(ok), nil
	}
}

func bPrompt(ctx *Ctx) Native {
	return func(args []val.Value) (val.Value, error) {
		return gui.Prompt(argText(args, 0, ""), argText(args, 1, ""))
	}
}
