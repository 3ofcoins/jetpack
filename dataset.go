package main

import "fmt"
import "log"
import "sort"
import "strconv"
import "strings"

import "github.com/3ofcoins/go-zfs"

type Dataset struct {
	*zfs.Dataset
}

func GetDataset(name string) Dataset {
	ds, err := zfs.GetDataset(name)
	if err != nil {
		log.Println("ERROR:", strings.Replace(err.Error(), "\n", " | ", -1))
	}
	return Dataset{ds}
}

func (ds Dataset) Exists() bool {
	return ds.Dataset != nil
}

type JailParameter struct {
	Name, Value string
}

func (jp JailParameter) String() string {
	if jp.Value == "" {
		return jp.Name
	} else {
		return fmt.Sprintf("%v=%#v", jp.Name, jp.Value)
	}
}

func (ds Dataset) JailParameters() []JailParameter {
	keys := make([]string, 0, len(ds.Properties))
	for k := range ds.Properties {
		if strings.HasPrefix(k, "jail:") {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	rv := make([]JailParameter, len(keys))
	for i, k := range keys {
		v := ds.Properties[k]
		if strings.HasPrefix(v, "\"") {
			if uv, err := strconv.Unquote(v); err != nil {
				log.Println("ERROR:", err)
			} else {
				v = uv
			}
		}
		rv[i] = JailParameter{k[5:], v}
	}
	return rv
}

func (ds Dataset) SetProperties(properties map[string]string) error {
	for n, v := range properties {
		if err := ds.SetProperty(n, v); err != nil {
			return err
		}
	}
	return nil
}