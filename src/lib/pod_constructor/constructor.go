package pod_constructor

import (
	"errors"
	"fmt"
	"net/url"
	"path"
	"strings"

	"github.com/appc/spec/discovery"
	"github.com/appc/spec/schema"
	"github.com/appc/spec/schema/types"
)

type constructor struct {
	pm   *schema.PodManifest
	args []string
	pos  int
	err  error
}

func newConstructor(pm *schema.PodManifest, args []string) *constructor {
	if pm == nil {
		pm = schema.BlankPodManifest()
	}
	return &constructor{pm, args, 0, nil}
}

func (c *constructor) parse() (*schema.PodManifest, error) {
	for state := stBOA; state != nil && c.err == nil; {
		if c.pos >= len(c.args) {
			// EOF
			break
		}
		state = state(c)
	}
	return c.pm, c.err
}

func (c *constructor) cur() string {
	return c.args[c.pos]
}

func (c *constructor) substitute(arg string) {
	c.args[c.pos] = arg
}

func (c *constructor) next() string {
	rv := c.args[c.pos]
	c.pos += 1
	return rv
}

func (c *constructor) app() *schema.RuntimeApp {
	if len(c.pm.Apps) == 0 {
		return nil
	} else {
		return &c.pm.Apps[len(c.pm.Apps)-1]
	}
}

func ConstructPodManifest(pm *schema.PodManifest, args []string) (*schema.PodManifest, error) {
	return newConstructor(pm, args).parse()
}

var ENOPARSE = errors.New("Invalid pod manifest spec")

type ErrFlag string

func (e ErrFlag) Error() string {
	return fmt.Sprintf("Unknown flag %#v", e)
}

func stFINI(*constructor) stateFn { return nil }
func stBOA(*constructor) stateFn  { return stGlobalFlag }

type stateFn func(*constructor) stateFn

func stDispatchFlag(c *constructor, table map[string]stateFn) stateFn {
	arg := c.cur()
	if arg[0] != '-' || len(arg) == 1 {
		// Not a flag
		return nil
	}
	arg = arg[1:]
	for name, stNext := range table {
		if arg == name {
			c.next()
			return stNext
		}
		if strings.HasPrefix(arg, name) {
			arg = arg[len(name):]
			if arg[0] == '=' {
				arg = arg[1:]
			}
			c.substitute(arg)
			return stNext
		}
	}
	c.err = ErrFlag(c.cur())
	return stFINI
}

func stGlobalFlag(c *constructor) stateFn {
	if sw := stDispatchFlag(c, map[string]stateFn{
		"a": stAnnotation,
		"v": stVolume,
		// "i": stIsolator,
		// "p": stExposedPort
	}); sw != nil {
		return sw
	}

	return stImage
}

func stAnnotation(c *constructor) stateFn {
	if splut := strings.SplitN(c.cur(), "=", 2); len(splut) < 2 {
		c.err = ENOPARSE
		return nil
	} else if name, err := types.NewACIdentifier(splut[0]); err != nil {
		c.err = err
		return nil
	} else if app := c.app(); app == nil {
		c.pm.Annotations.Set(*name, splut[1])
		c.next()
		return stGlobalFlag
	} else {
		app.Annotations.Set(*name, splut[1])
		c.next()
		return stAppFlag
	}
}

func stVolume(c *constructor) stateFn {
	// Transform sanely formatted string to Rocket/appc format. Should
	// we even support it?
	arg := c.cur()

	if !strings.ContainsRune(arg, ',') {
		if pieces := strings.SplitN(arg, ":", 2); len(pieces) == 1 {
			arg += ",kind=empty"
		} else {
			arg = fmt.Sprintf("%v,kind=host,source=%v", pieces[0], url.QueryEscape(pieces[1]))
		}
	}
	if arg[0] == '-' {
		arg = arg[1:] + ",readOnly=true"
	}

	if v, err := types.VolumeFromString(arg); err != nil {
		c.err = err
		return nil
	} else {
		for _, vol := range c.pm.Volumes {
			if vol.Name == v.Name {
				c.err = fmt.Errorf("Volume %v already defined: %v", v.Name, vol)
				return nil
			}
		}
		c.pm.Volumes = append(c.pm.Volumes, *v)
		c.next()
		return stGlobalFlag
	}
}

func stApp(c *constructor) stateFn {
	var rta schema.RuntimeApp
	// If name is "-", we leave app's name empty for stImage to fill in
	// the default
	if name := c.next(); name != "-" {
		if err := rta.Name.Set(name); err != nil {
			c.err = err
			return nil
		}
	}
	c.pm.Apps = append(c.pm.Apps, rta)
	return stImage
}

func stImage(c *constructor) stateFn {
	c.pm.Apps = append(c.pm.Apps, schema.RuntimeApp{})

	imgName := c.next()
	rta := c.app()

	// First try if image is a raw hash
	if hash, err := types.NewHash(imgName); err == nil {
		rta.Image.ID = *hash
	} else if da, err := discovery.NewAppFromString(imgName); err != nil {
		c.err = err
		return nil
	} else if labels, err := types.LabelsFromMap(da.Labels); err != nil {
		c.err = err
		return nil
	} else {
		rta.Image.Name = &da.Name
		rta.Image.Labels = labels
	}

	// Try to fill in default app name from image's basename. Don't try too hard.
	if rta.Image.Name != nil {
		if appName, err := types.SanitizeACName(path.Base(rta.Image.Name.String())); err == nil {
			if appACName, err := types.NewACName(appName); err == nil {
				rta.Name = *appACName
			}
		}
	}

	return stAppFlag
}

func stAppFlag(c *constructor) stateFn {
	if sw := stDispatchFlag(c, map[string]stateFn{
		"n": stName,
		"a": stAnnotation,
		"m": stMount,
		// TODO: app override
	}); sw != nil {
		return sw
	}

	return stImage
}

func stName(c *constructor) stateFn {
	rta := c.app()
	if name, err := types.NewACName(c.next()); err != nil {
		c.err = err
		return nil
	} else {
		rta.Name = *name
	}
	return stAppFlag
}

func stMount(c *constructor) stateFn {
	app := c.app()

	if app == nil {
		c.err = errors.New("CAN'T HAPPEN")
		return nil
	}

	splut := strings.SplitN(c.next(), ":", 2)
	if len(splut) < 2 {
		splut = append(splut, splut[0])
	}

	mnt := schema.Mount{}
	if err := mnt.MountPoint.Set(splut[0]); err != nil {
		c.err = err
		return nil
	}
	if err := mnt.Volume.Set(splut[1%len(splut)]); err != nil {
		c.err = err
		return nil
	}

	app.Mounts = append(app.Mounts, mnt)
	return stAppFlag
}
