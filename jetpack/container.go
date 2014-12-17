package jetpack

import "encoding/json"
import "fmt"
import "io/ioutil"
import "os"

import "github.com/appc/spec/schema"
import "github.com/juju/errors"

type Container struct {
	Dataset  `json:"-"`
	Manifest schema.ContainerRuntimeManifest `json:"-"`
}

func NewContainer(ds *Dataset) *Container {
	return &Container{Dataset: *ds}
}

func GetContainer(ds *Dataset) (*Container, error) {
	c := NewContainer(ds)
	if err := c.Load(); err != nil {
		return nil, err
	} else {
		return c, nil
	}
}

func (c *Container) IsEmpty() bool {
	_, err := os.Stat(c.Path("manifest"))
	return os.IsNotExist(err)
}

func (c *Container) IsLoaded() bool {
	return !c.Manifest.ACVersion.Empty()
}

func (c *Container) Load() error {
	if c.IsLoaded() {
		return errors.New("Already loaded")
	}

	if c.IsEmpty() {
		return errors.New("Container is empty")
	}

	if err := c.readManifest(); err != nil {
		return errors.Trace(err)
	}

	return nil
}

func (c *Container) readManifest() error {
	manifestJSON, err := ioutil.ReadFile(c.Path("manifest"))
	if err != nil {
		return errors.Trace(err)
	}

	if err = json.Unmarshal(manifestJSON, &c.Manifest); err != nil {
		return errors.Trace(err)
	}

	return nil
}

func (c *Container) Save() error {
	if manifestJSON, err := json.Marshal(c.Manifest); err != nil {
		return errors.Trace(err)
	} else {
		return ioutil.WriteFile(c.Path("manifest"), manifestJSON, 0400)
	}
}

func (c *Container) String() string {
	return fmt.Sprintf("#<Container %v %v>",
		c.Manifest.UUID, c.Manifest.Apps[0].Name)
}

func (c *Container) PPPrepare() interface{} {
	return map[string]interface{}{
		"Manifest": c.Manifest,
		"Path":     c.Mountpoint,
		"Dataset":  c.Name,
	}
}
