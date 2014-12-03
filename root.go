package main

type rootDS struct{ Dataset }

var ZFSRoot = "zroot/zjail"
var Root rootDS

func init() {
	Root = rootDS{GetDataset(ZFSRoot)}
}

func (r rootDS) Children() ([]Jail, error) {
	children, err := r.Dataset.Children(0)
	rv := make([]Jail, len(children))
	for i := range children {
		rv[i] = Jail{Dataset{children[i]}}
	}
	return rv, err
}
