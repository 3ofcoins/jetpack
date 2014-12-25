package config

import "encoding/json"
import "io/ioutil"
import "os"
import "strconv"
import "strings"

type Config map[string]string

func NewConfig() Config {
	return make(Config)
}

func (c Config) UpdateFrom(other Config, prefix string) {
	if prefix != "" && prefix[len(prefix)-1] != '/' {
		prefix += "/"
	}
	for k, v := range other {
		c[prefix+k] = v
	}
}

func (c Config) LoadFromFile(path string) error {
	if bytes, err := ioutil.ReadFile(path); err != nil {
		return err
	} else {
		return json.Unmarshal(bytes, &c)
	}
}

func (c Config) SaveToFile(path string, perm os.FileMode) error {
	if bytes, err := json.Marshal(c); err != nil {
		return err
	} else {
		return ioutil.WriteFile(path, bytes, perm)
	}
}

// Try loading config from file. If `path' does not exist, save
// current contents as defaults.
func (c Config) LoadOrSave(path string, perm os.FileMode) error {
	if err := c.LoadFromFile(path); err != nil {
		if os.IsNotExist(err) {
			return c.SaveToFile(path, perm)
		}
		return err
	}
	return nil
}

func (c Config) LoadArguments(args ...string) {
	for _, arg := range args {
		if pieces := strings.SplitN(arg, "=", 2); len(pieces) == 1 {
			// Flag: +truth, +nested/truth, -false, -nested/false, truth, nested/truth, nofalse, nested/nofalse
			k := arg
			v := "true"
			switch {
			case k[0] == '+':
				k = k[1:]
			case k[0] == '-':
				v = "false"
				k = k[1:]
			default:
				pieces := strings.Split(k, "/")
				lastIdx := len(pieces) - 1
				if strings.HasPrefix(pieces[lastIdx], "no") {
					v = "false"
					pieces[lastIdx] = pieces[lastIdx][2:]
					k = strings.Join(pieces, "/")
				}
			}
			c[k] = v
		} else {
			c[pieces[0]] = pieces[1]
		}
	}
}

func (c Config) GetString(name string) (string, error) {
	if v, ok := c[name]; ok {
		return v, nil
	}
	return "", ErrKeyNotFound
}

func (c Config) GetBool(name string) (bool, error) {
	if v, ok := c[name]; ok {
		switch strings.ToLower(v) {
		case "true", "t", "yes", "on":
			return true, nil
		case "false", "f", "no", "off":
			return false, nil
		default:
			return false, invalidValue("bool", v, nil)
		}
	}
	return false, ErrKeyNotFound
}

func (c Config) GetInt(name string) (int, error) {
	if v, ok := c[name]; ok {
		if iv, err := strconv.Atoi(v); err != nil {
			return 0, invalidValue("int", v, err)
		} else {
			return iv, nil
		}
	}
	return 0, ErrKeyNotFound
}

func (c Config) GetSubtree(prefix string) Config {
	if prefix[len(prefix)-1] != '/' {
		prefix += "/"
	}
	i := len(prefix)
	rv := make(Config)
	for k, v := range c {
		if strings.HasPrefix(k, prefix) {
			rv[k[i:]] = v
		}
	}
	return rv
}

func (c Config) Delete(name string) {
	if name[len(name)-1] == '/' {
		for k := range c {
			if strings.HasPrefix(k, name) {
				delete(c, k)
			}
		}
	} else {
		delete(c, name)
	}
}

func (c Config) ExtractSubtree(prefix string) Config {
	if prefix[len(prefix)-1] != '/' {
		prefix += "/"
	}
	rv := c.GetSubtree(prefix)
	c.Delete(prefix)
	return rv
}
