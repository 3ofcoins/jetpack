package jetpack

import "fmt"
import "log"
import "sort"
import "strconv"
import "strings"

import "github.com/3ofcoins/go-zfs"

type Dataset struct {
	*zfs.Dataset
}

//DONE var ZeroDataset = Dataset{nil}

func GetDataset(name string) (Dataset, error) {
	//DONE 	ds, err := zfs.GetDataset(name)
	//DONE 	return Dataset{ds}, err
	return Datase{}, nil
}

func (ds Dataset) Exists() bool {
	//IRRELEVANT 	return ds.Dataset != nil
	return false
}

type JailParameter struct {
	Name, Value string
}

func (jp JailParameter) String() string {
	//IRRELEVANT 	if jp.Value == "" {
	//IRRELEVANT 		return jp.Name
	//IRRELEVANT 	} else {
	//IRRELEVANT 		return fmt.Sprintf("%v=%#v", jp.Name, jp.Value)
	//IRRELEVANT 	}
	return ""
}

func (ds Dataset) JailParameters() []JailParameter {
	//IRRELEVANT 	keys := make([]string, 0, len(ds.Properties))
	//IRRELEVANT 	for k := range ds.Properties {
	//IRRELEVANT 		if strings.HasPrefix(k, "jetpack:jail:") {
	//IRRELEVANT 			keys = append(keys, k)
	//IRRELEVANT 		}
	//IRRELEVANT 	}
	//IRRELEVANT 	sort.Strings(keys)
	//IRRELEVANT 	rv := make([]JailParameter, len(keys))
	//IRRELEVANT 	for i, k := range keys {
	//IRRELEVANT 		v := ds.Properties[k]
	//IRRELEVANT 		if strings.HasPrefix(v, "\"") {
	//IRRELEVANT 			if uv, err := strconv.Unquote(v); err != nil {
	//IRRELEVANT 				log.Println("ERROR:", err)
	//IRRELEVANT 			} else {
	//IRRELEVANT 				v = uv
	//IRRELEVANT 			}
	//IRRELEVANT 		}
	//IRRELEVANT 		rv[i] = JailParameter{k[len("jetpack:jail:"):], v}
	//IRRELEVANT 	}
	//IRRELEVANT 	return rv
	return nil
}

func (ds Dataset) SetProperties(properties map[string]string) error {
	//IRRELEVANT 	for n, v := range properties {
	//IRRELEVANT 		if err := ds.SetProperty(n, v); err != nil {
	//IRRELEVANT 			return err
	//IRRELEVANT 		}
	//IRRELEVANT 	}
	return nil
}

func (ds Dataset) Snapshots() []Dataset {
	//DONE 	snaps, err := ds.Dataset.Snapshots()
	//DONE 	if err != nil {
	//DONE 		log.Fatalln("ERROR:", err)
	//DONE 	}
	//DONE 	rv := make([]Dataset, len(snaps))
	//DONE 	for i, snap := range snaps {
	//DONE 		rv[i] = Dataset{snap}
	//DONE 	}
	//DONE 	return rv
	return nil
}

func (ds Dataset) String() string {
	return ds.Name
}
