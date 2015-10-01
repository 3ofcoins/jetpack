package jetpack

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/magiconair/properties"
)

const version = "0.0.1"

// Parameters that can be overridden build-time (https://www.socketloop.com/tutorials/golang-setting-variable-value-with-ldflags)
var prefix = "/usr/local"

// Default config
var defaultConfig = []byte(fmt.Sprintf(`
ace.jailConf.osrelease=10.2-RELEASE
ace.jailConf.securelevel=2
allow.autodiscovery = on
allow.http = off
allow.no-signature = off
debug = off
images.aci.compression=xz
images.zfs.atime=off
images.zfs.compress=lz4
jail.interface = lo1
jail.namePrefix = jetpack/
mds.port = 1104
mds.user = _jetpack
path.libexec = ${path.prefix}/libexec/jetpack
path.share = ${path.prefix}/share/jetpack
path.prefix = %v
root.zfs = zroot/jetpack
root.zfs.mountpoint = /var/jetpack
`,
	prefix))

var ConfigPath = filepath.Join(prefix, "etc/jetpack.conf")

type propertiesFlag map[string]string

func (pfl propertiesFlag) String() string {
	return "VAR[=VALUE]..."
}

func (pfl propertiesFlag) Set(v string) error {
	pieces := strings.SplitN(v, "=", 2)
	if len(pieces) == 1 {
		pfl[v] = "on"
	} else {
		pfl[pieces[0]] = pieces[1]
	}
	return nil
}

var ConfigOverrides = make(propertiesFlag)

func init() {
	if cf := os.Getenv("JETPACK_CONF"); cf != "" {
		ConfigPath = cf
	}
	flag.StringVar(&ConfigPath, "config", ConfigPath, "Path to configuration file")
	flag.Var(&ConfigOverrides, "o", "Override configuration properties")
}

func ConfigFlags() []string {
	rv := make([]string, 1+len(ConfigOverrides))
	rv[0] = fmt.Sprintf("-config=%v", configPath)
	i := 1
	for k, v := range ConfigOverrides {
		rv[i] = fmt.Sprintf("-o=%v=%v", k, v)
		i = i + 1
	}
	return rv
}

var configProperties *properties.Properties
var configPath string

func Config() *properties.Properties {
	if configProperties == nil {
		cfg := defaultConfig

		cfgPath, err := filepath.Abs(ConfigPath)
		if err != nil {
			panic(err)
		}

		if cfgFile, err := ioutil.ReadFile(cfgPath); os.IsNotExist(err) {
			// pass
		} else if err != nil {
			panic(err)
		} else {
			cfg = append(cfg, cfgFile...)
		}
		if props, err := properties.Load(cfg, properties.UTF8); err != nil {
			panic(err)
		} else {
			for k, v := range ConfigOverrides {
				if _, _, err := props.Set(k, v); err != nil {
					panic(err)
				}
			}
			configPath = cfgPath
			configProperties = props
		}
	}
	return configProperties
}

func ConfigPrefix(prefix string) map[string]string {
	rv := make(map[string]string)
	pl := len(prefix)
	pp := Config().FilterPrefix(prefix)
	for _, pk := range pp.Keys() {
		if pv, ok := pp.Get(pk); ok && pv != "" {
			rv[pk[pl:]] = pv
		}
	}
	return rv
}

func Version() string {
	if revision, ok := Config().Get("version.git"); ok {
		return fmt.Sprintf("%v+git(%v)", version, revision)
	} else {
		return version
	}
}
