package std

import (
	"fmt"

	"github.com/name-cho/uniland-go/internal/gui"
	"github.com/name-cho/uniland-go/internal/interp/val"
	"github.com/name-cho/uniland-go/internal/pyops"
)

func guiGate() error {
	if !gui.Available() {
		return gui.ErrUnavailable
	}
	return nil
}

func widgetOf(m *val.Map, lineName string) (gui.Widget, error) {
	t, ok := m.Get("type").(val.Str)
	if !ok {
		return gui.Widget{}, fmt.Errorf("%s(): each widget needs a string \"type\"", lineName)
	}
	w := gui.Widget{Type: string(t)}
	switch w.Type {
	case "label":
		w.Text = pyops.PyStr(m.Get("text"))
	case "separator":

	case "entry":
		w.Text = pyops.PyStr(m.Get("text"))
		if k, ok := m.Get("key").(val.Str); ok {
			w.Key = string(k)
		}
	case "check":
		w.Text = pyops.PyStr(m.Get("text"))
		w.Checked = pyops.Truthy(m.Get("checked"))
		if k, ok := m.Get("key").(val.Str); ok {
			w.Key = string(k)
		}
	case "button":
		w.Text = pyops.PyStr(m.Get("text"))
		fn := m.Get("onclick")
		if fn != nil && !isNullValue(fn) {
			w.OnClick = fn
		}
	default:
		return gui.Widget{}, fmt.Errorf("%s(): unknown widget type '%s'", lineName, string(t))
	}
	return w, nil
}

func bGUIWindow(ctx *Ctx) Native {
	return func(args []val.Value) (val.Value, error) {
		if err := guiGate(); err != nil {
			return nil, err
		}
		if len(args) < 2 || len(args) > 3 {
			return nil, fmt.Errorf("gui_window() expects (title, widgets[, opts])")
		}
		title, ok := args[0].(val.Str)
		if !ok {
			return nil, fmt.Errorf("gui_window() first argument must be a string, not %s", pyops.TypeName(args[0]))
		}
		widgets, ok := args[1].(*val.List)
		if !ok {
			return nil, fmt.Errorf("gui_window() second argument must be an array, not %s", pyops.TypeName(args[1]))
		}
		spec := gui.WindowSpec{Title: string(title)}
		for _, item := range widgets.Items {
			m, ok := item.(*val.Map)
			if !ok {
				return nil, fmt.Errorf("gui_window(): each widget must be an object, not %s", pyops.TypeName(item))
			}
			w, err := widgetOf(m, "gui_window")
			if err != nil {
				return nil, err
			}
			spec.Widgets = append(spec.Widgets, w)
		}
		if len(args) == 3 {
			opts, ok := args[2].(*val.Map)
			if !ok {
				return nil, fmt.Errorf("gui_window() opts must be an object, not %s", pyops.TypeName(args[2]))
			}
			if v := opts.Get("width"); v != nil {
				n, err := pyops.IntOf(v)
				if err != nil {
					return nil, err
				}
				if n.IsInt64() {
					spec.Width = int(n.Int64())
				}
			}
			if v := opts.Get("height"); v != nil {
				n, err := pyops.IntOf(v)
				if err != nil {
					return nil, err
				}
				if n.IsInt64() {
					spec.Height = int(n.Int64())
				}
			}
		}
		out, err := gui.RunWindow(spec, gui.CallFn(ctx.Call))
		if err != nil {
			return nil, err
		}
		m := val.NewMap()
		for k, v := range out {
			m.Set(k, v)
		}
		return m, nil
	}
}

func bGUIAlert(ctx *Ctx) Native {
	return func(args []val.Value) (val.Value, error) {
		if err := guiGate(); err != nil {
			return nil, err
		}
		if err := need(args, 2, "gui_alert"); err != nil {
			return nil, err
		}
		title, ok := args[0].(val.Str)
		if !ok {
			return nil, fmt.Errorf("gui_alert() first argument must be a string, not %s", pyops.TypeName(args[0]))
		}
		if err := gui.Alert(string(title), pyops.PyStr(args[1])); err != nil {
			return nil, err
		}
		return val.Nul, nil
	}
}

func bGUIAsk(ctx *Ctx) Native {
	return func(args []val.Value) (val.Value, error) {
		if err := guiGate(); err != nil {
			return nil, err
		}
		if len(args) < 2 || len(args) > 3 {
			return nil, fmt.Errorf("gui_ask() expects (title, text[, initial])")
		}
		title, ok := args[0].(val.Str)
		if !ok {
			return nil, fmt.Errorf("gui_ask() first argument must be a string, not %s", pyops.TypeName(args[0]))
		}
		initial := ""
		if len(args) == 3 {
			if ini, ok2 := args[2].(val.Str); ok2 {
				initial = string(ini)
			} else {
				return nil, fmt.Errorf("gui_ask() initial must be a string, not %s", pyops.TypeName(args[2]))
			}
		}
		v, err := gui.Ask(string(title), pyops.PyStr(args[1]), initial)
		if err != nil {
			return nil, err
		}

		if sv, ok := v.(val.Str); ok {
			return sv, nil
		}
		return val.Nul, nil
	}
}

func bGUIConfirm(ctx *Ctx) Native {
	return func(args []val.Value) (val.Value, error) {
		if err := guiGate(); err != nil {
			return nil, err
		}
		if err := need(args, 2, "gui_confirm"); err != nil {
			return nil, err
		}
		title, ok := args[0].(val.Str)
		if !ok {
			return nil, fmt.Errorf("gui_confirm() first argument must be a string, not %s", pyops.TypeName(args[0]))
		}
		okb, err := gui.Confirm(string(title), pyops.PyStr(args[1]))
		if err != nil {
			return nil, err
		}
		return val.Bool(okb), nil
	}
}
