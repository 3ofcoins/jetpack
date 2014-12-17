package ui

import "fmt"
import "math"
import "reflect"

type Shower interface {
	Show(*UI)
}

type Summarizer interface {
	Summary() string
}

type ObjShower func(interface{}, *UI)

var TypeShowers = make(map[reflect.Type]ObjShower)

func SetShower(obj interface{}, shower ObjShower) {
	typ := reflect.TypeOf(obj)
	if typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}
	TypeShowers[typ] = shower
}

func StringerShower(obj interface{}, ui *UI) {
	ui.Sayf(": %v", obj)
}

func EscapedValueShower(obj interface{}, ui *UI) {
	ui.Sayf(": %#v", obj)
}

func NoShower(interface{}, *UI) {}

func (ui *UI) Show(obj interface{}) {
	ui.show(reflect.ValueOf(obj))
}

func (ui *UI) RawShow(obj interface{}) {
	ui.rawShow(reflect.ValueOf(obj))
}

func (ui *UI) show(v reflect.Value) {
	if !v.IsValid() {
		// pass
	} else if shower, ok := v.Interface().(Shower); ok {
		shower.Show(ui)
	} else if shower, ok := TypeShowers[v.Type()]; ok {
		shower(v.Interface(), ui)
	} else {
		ui.rawShow(v)
	}
}

func (ui *UI) rawShow(v reflect.Value) {
	switch v.Kind() {
	case reflect.Invalid: // pass
	case reflect.Ptr:
		ui.show(v.Elem())
	case reflect.Bool,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
		reflect.Float32, reflect.Float64,
		reflect.Complex64, reflect.Complex128,
		reflect.String:
		ui.Sayf(": %v", v.Interface())
	case reflect.Struct:
		for i, t, nf := 0, v.Type(), v.NumField(); i < nf; i++ {
			ft := t.Field(i)
			fv := v.Field(i)
			if ft.PkgPath != "" {
				continue
			}
			if ui.IsIndented() {
				ui.Indentf(".%v", ft.Name)
			} else {
				ui.Indent(ft.Name)
			}
			ui.show(fv)
			ui.Dedent()
		}
	case reflect.Slice, reflect.Array:
		l := v.Len()
		ifmt := fmt.Sprintf("[%%%dd]", int(math.Ceil(math.Log10(float64(l+1)))))
		for i := 0; i < l; i++ {
			ui.Indentf(ifmt, i+1)
			ui.show(v.Index(i))
			ui.Dedent()
		}
	case reflect.Map:
		for _, key := range v.MapKeys() {
			ui.Indentf("[%#v]", key.Interface())
			ui.show(v.MapIndex(key))
			ui.Dedent()
		}
	default:
		ui.Sayf("%v:%#v", v.Kind(), v.Interface())
	}
}

func (ui *UI) Summarize(obj interface{}) {
	ui.summarize(reflect.ValueOf(obj))
}

func (ui *UI) summarize(v reflect.Value) {
	if sum, ok := v.Interface().(Summarizer); ok {
		ui.Say(sum.Summary())
	} else {
		switch v.Kind() {
		case reflect.Invalid: // pass
		case reflect.Ptr:
			ui.summarize(v.Elem())

		case reflect.Slice, reflect.Array:
			l := v.Len()
			ifmt := fmt.Sprintf("%%%dd) ", int(math.Ceil(math.Log10(float64(l+1)))))
			for i := 0; i < l; i++ {
				ui.Indentf(ifmt, i+1)
				ui.summarize(v.Index(i))
				ui.Dedent()
			}
		case reflect.Map:
			for _, key := range v.MapKeys() {
				ui.Indentf("%v) ", key.Interface())
				ui.summarize(v.MapIndex(key))
				ui.Dedent()
			}
		default:
			ui.Sayf("%v", v.Interface())
		}
	}
}
