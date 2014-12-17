package jetpack

import "code.google.com/p/go-uuid/uuid"
import "github.com/appc/spec/schema/types"

import "github.com/3ofcoins/jetpack/ui"

func labelsShower(obj interface{}, ui *ui.UI) {
	for _, label := range obj.(types.Labels) {
		ui.Sayf("[%v]: %v", label.Name, label.Value)
	}
}

func init() {
	ui.SetShower(types.ACKind(""), ui.NoShower)
	ui.SetShower(types.Hash{}, ui.StringerShower)
	ui.SetShower(types.Labels{}, labelsShower)
	ui.SetShower(types.SemVer{}, ui.StringerShower)
	ui.SetShower(types.UUID{}, ui.StringerShower)
	ui.SetShower(uuid.UUID{}, ui.StringerShower)
}
